package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/jpoehls/gophermail"
	google "github.com/kevinburke/google-oauth-handler"
	"github.com/kevinburke/rest"
	"github.com/kevinburke/semaphore"
	"github.com/russross/blackfriday"
	"golang.org/x/sync/errgroup"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

var sema *semaphore.Semaphore

func init() {
	sema = semaphore.New(2)
}

type Recipient struct {
	Address     *mail.Address
	CC          []mail.Address
	OpeningLine string // "Supervisor Kim"
}

type Group struct {
	Recipients []*Recipient
	// Unique ID for this group
	ID string
	// Appears in the UI to represent this group
	Name string
}

type Mailer struct {
	Groups    map[string]*Group
	Logger    log.Logger
	secretKey *[32]byte
}

func (m *Mailer) sendMail(w http.ResponseWriter, r *http.Request, auth *google.Auth) {
	// TODO check csrf
	subject := strings.TrimSpace(r.FormValue("subject"))
	body := strings.TrimSpace(r.FormValue("body"))
	id := r.FormValue("group_id")
	if subject == "" {
		FlashError(w, "Please provide a subject", m.secretKey)
		http.Redirect(w, r, "/", 302)
		return
	}
	if body == "" {
		FlashError(w, "Please provide a message body", m.secretKey)
		http.Redirect(w, r, "/", 302)
		return
	}
	var group *Group
	if id == "test" {
		group = &Group{
			Recipients: []*Recipient{
				&Recipient{Address: auth.Email, OpeningLine: "Hi test"},
			},
		}
	} else {
		var ok bool
		if group, ok = m.Groups[id]; !ok {
			rest.ServerError(w, r, fmt.Errorf("unknown group %s", id))
			return
		}
	}
	srv, err := gmail.New(auth.Client)
	if err != nil {
		rest.ServerError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	eg, errctx := errgroup.WithContext(ctx)
	for _, recipient := range group.Recipients {
		body := body
		to := *recipient
		eg.Go(func() error {
			line := strings.TrimSpace(to.OpeningLine)
			if !strings.HasSuffix(line, ",") {
				line = line + ","
			}
			html := line + "<br />" + string(blackfriday.MarkdownCommon([]byte(body)))
			body := line + "\n\n" + body
			msg := &gophermail.Message{
				From:     *auth.Email,
				To:       []mail.Address{*to.Address},
				Cc:       recipient.CC,
				Subject:  subject,
				Body:     body,
				HTMLBody: string(html),
			}
			raw, err := msg.Bytes()
			if err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				call := srv.Users.Messages.Send(auth.Email.Address, &gmail.Message{
					Raw: base64.URLEncoding.EncodeToString(raw),
				})
				call = call.Context(errctx)
				sema.Acquire()
				_, doErr := call.Do()
				sema.Release()
				if doErr == nil {
					m.Logger.Info("Successfully sent message", "from", auth.Email.String(), "to", to.Address.String())
					return nil
				}
				if doErr == context.Canceled {
					return doErr
				}
				switch terr := doErr.(type) {
				case *googleapi.Error:
					switch terr.Code {
					case 429, 500:
						// TODO figure out whether this actually sends the
						// message
						dur := time.Duration(i+1) * 2 * time.Second
						m.Logger.Info("got retryable error", "err", terr, "code", terr.Code, "sleep_dur", dur)
						time.Sleep(dur)
						continue
					default:
						// We failed to send a message; it happens. Shouldn't block
						// sending of other emails, so log and return nil.
						m.Logger.Error("Error sending message", "from", auth.Email.String(),
							"to", to.Address.String(), "err", fmt.Sprintf("%#v", terr))
						return terr
					}
				default:
					// We failed to send a message; it happens. Shouldn't block
					// sending of other emails, so log and return nil.
					m.Logger.Error("Error sending message", "from", auth.Email.String(),
						"to", to.Address.String(), "err", fmt.Sprintf("%#v", terr))
					return terr
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		rest.ServerError(w, r, err)
		return
	}
	var word = "message"
	if len(group.Recipients) > 1 {
		word = "messages"
	}
	FlashSuccess(w, fmt.Sprintf("Sent %d %s. They will appear in your Sent folder shortly", len(group.Recipients), word), m.secretKey)
	http.Redirect(w, r, "/", 302)
}
