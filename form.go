package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	log "github.com/inconshreveable/log15"
	"github.com/jpoehls/gophermail"
	google "github.com/kevinburke/google-oauth-handler"
	"github.com/kevinburke/rest"
	"github.com/russross/blackfriday"
	"golang.org/x/sync/errgroup"
	gmail "google.golang.org/api/gmail/v1"
)

type Recipient struct {
	Address     *mail.Address
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
	eg, errctx := errgroup.WithContext(r.Context())
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
				Subject:  subject,
				Body:     body,
				HTMLBody: string(html),
			}
			raw, err := msg.Bytes()
			if err != nil {
				return err
			}
			call := srv.Users.Messages.Send(auth.Email.Address, &gmail.Message{
				Raw: base64.URLEncoding.EncodeToString(raw),
			})
			call = call.Context(errctx)
			_, doErr := call.Do()
			if doErr == nil {
				m.Logger.Info("Successfully sent message", "from", auth.Email.String(), "to", to.Address.String())
			}
			return doErr
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