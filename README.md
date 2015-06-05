# CaptainHook
CaptainHook is a Github Webhook IRC notifier bot. 
## Why?
Yes, Github already has a perfectly capable IRC 'service' for repositories, however:
- Uses [organisation-wide webhooks](https://github.com/blog/1933-introducing-organization-webhooks) meaning that all repositories' events are received by default instead of manually enabling/configuring each repo
- Allows easier filtering of event types (you can select them from the webhook config page,
whereas Github's IRC service only allows this level of configuration using the API)
- 
- More fun

## Getting
- Set up a Go environment, if you haven't already (and why not?)
- `go get github.com/UniversityRadioYork/CaptainHook`
- `go install` (or just `go build` to run from the project directory)

## Running
- `cp config.toml.example config.toml && $EDITOR config.toml`
- `captainhook`
