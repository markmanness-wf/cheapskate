package sdk

type Service string

// TODO: Make this a dynamically configured file?
const (
	ServiceCerberus         Service = "cerberus"
	ServiceLinkingApi       Service = "linking-api"
	ServiceLinkingDirectory Service = "linking-directory"
	ServiceLinkingNotifier  Service = "linking-notifier"
)
