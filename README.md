# CaptainHook
CaptainHook is a Github Webhook IRC notifier bot. 
## Why?
Github has a perfectly capable IRC service already for repositories, however
processing webhooks yourself allows you to use
[organisation-wide webhooks](https://github.com/blog/1933-introducing-organization-webhooks) meaning that all repositories' events 
are received by default, and it also allows for easier filtering of event types (you can select them from the webhook config page,
whereas the IRC service only allows this level of configuration using the API)
Finally, it allows a tighter control on the content of IRC notifications, and interaction with the bot too (telling it to ignore 
certain repositories/events for example)

## Getting
Assuming you have a Go environment, `go get github.com/UniversityRadioYork/CaptainHook`. Then `go install` / `go build` as desired.

## Running
- Copy the example config file and edit it
- `captainhook -c /path/to/config.toml`

## Interactions
Currently, none. Apart from a silly `s/my/muh/`, which will probably get old quickly
