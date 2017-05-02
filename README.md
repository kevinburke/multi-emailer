# multi-emailer

This is a tool for sending out multiple "personal" emails at the same time.
They'll be sent from your personal Gmail account, and the recipient's name will
be attached to the top of each email, so it looks like you hand wrote it (unless
you inspect the email *very* closely).

Here's a screenshot:

<img src="https://monosnap.com/file/y8iqP37afiCUA1lN0xhoifYWsXP0Bx.png">

## Installation

- Download the right `multi-emailer` binary for your platform from [the releases
page][releases], and copy it to the server.

- Rename `config.sample.yml` to `config.yml` and populate it with values that are
appropriate - you'll need a Google Client ID and Secret.

- [Enable the GMail API][enable] for the project you created.

[enable]: https://console.developers.google.com/apis/api/gmail.googleapis.com/overview

- Add the groups of people you want to email. The `email` key should follow this
format: `"First Last" <email@domain.com>`. You can also provide a plain email
address - `email@domain.com`. The `opening_line` should be the first line of
the email to that person - "Dear X". We'll add the comma and the rest of the
message.

- Start the server: `multi-emailer --config=/path/to/config.yml`. That's it!
Logs are sent to stderr and can be redirected from there.

## Usage

When users visit the site they'll be redirected to a Google approval page. This
page will ask them for permission to send emails on their behalf. Then they'll
be redirected and can type away!

[releases]: https://github.com/kevinburke/multi-emailer/releases
