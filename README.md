# multi-emailer

This is a tool for sending out multiple "personal" emails at the same time.
They'll be sent from your personal Gmail account, and the recipient's name will
be attached to the top of each email, so it looks like you hand wrote it (unless
you inspect the email *very* closely).

Here's a screenshot:

<img src="https://monosnap.com/image/BgmNUzU60hk0uyxcC32cRBuhvJwiUN.png">

## Installation

- Download the right `multi-emailer` binary for your platform from [the releases
page][releases], and copy it to the server.

- Rename `config.sample.yml` to `config.yml` and populate it with values that
are appropriate - you'll need [a Google Client ID and Secret][google].

- [Enable the GMail API][enable] for the project you created.

[google]: https://github.com/kevinburke/logrole/blob/master/docs/google.md
[enable]: https://console.developers.google.com/apis/api/gmail.googleapis.com/overview

- Add the groups of people you want to email. The `email` key should follow this
format: `"First Last" <email@domain.com>`. You can also provide a plain email
address - `email@domain.com`. The `opening_line` should be the first line of
the email to that person - "Dear X". We'll add the comma and the rest of the
message. Here is an example:

    ```yaml
    groups:
      - id: sf-planning-commission
        name: SF Planning Commission
        recipients:
            - email: Rich Hillis <richhillissf@yahoo.com>
              opening_line: Commissioner Hillis
              cc:
                - Planning Commission Secretary <commissions.secretary@sfgov.org>
            - email: Dennis Richards <dennis.richards@sfgov.org>
              opening_line: Commissioner Richards
            - email: Rodney Fong <planning@rodneyfong.com>
              opening_line: Commissioner Fong
            - email: Christine Johnson <christine.d.johnson@sfgov.org>
              opening_line: Commissioner Johnson
            - email: Joel Koppel <joel.koppel@sfgov.org>
              opening_line: Commissioner Koppel
            - email: Myrna Melgar <myrna.melgar@sfgov.org>
              opening_line: Commissioner Melgar
            - email: Kathrin Moore <kathrin.moore@sfgov.org>
              opening_line: Commissioner Moore
    ```

- Start the server: `multi-emailer --config=/path/to/config.yml`. That's it!
Logs are sent to stderr and can be redirected from there.

#### Deploying to Google Cloud

Run `./bin/gae-deploy`. Google Cloud has trouble with the `vendor` directory.

#### Deploying to Other Platforms

You can download and install the binary directly:

```bash
curl --silent --location --output /usr/local/bin/multi-emailer https://github.com/kevinburke/multi-emailer/releases/download/1.7/multi-emailer-linux-amd64
chmod 755 /usr/local/bin/multi-emailer
```

Then run `/usr/local/bin/multi-emailer` in a directory with the config file and
the server should start as you expect.

You'll probably need to tweak the project to deploy to Heroku or elsewhere. I'd
like to help make that feasible. Please contact me directly - kev@inburke.com -
for assistance.

## Usage

When users visit the site they'll be redirected to a Google approval page. This
page will ask them for permission to send emails on their behalf. Then they'll
be redirected and can type away!

[releases]: https://github.com/kevinburke/multi-emailer/releases
