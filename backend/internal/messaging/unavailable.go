package messaging

import "context"

// UnavailableProvider denies all operations; it never uses another tenant's credentials.
type UnavailableProvider struct{ Err error }

func (p UnavailableProvider) Send(context.Context, OutgoingMessage) (SendResult, error) {
	return SendResult{}, p.Err
}
func (p UnavailableProvider) SendTemplate(context.Context, TemplateMessage) (SendResult, error) {
	return SendResult{}, p.Err
}
func (p UnavailableProvider) SendMenu(context.Context, string, string, []MenuOption) (SendResult, error) {
	return SendResult{}, p.Err
}
func (p UnavailableProvider) SendMedia(context.Context, OutgoingMedia) (SendResult, error) {
	return SendResult{}, p.Err
}
func (p UnavailableProvider) SendReaction(context.Context, string, string, string) error {
	return p.Err
}
func (p UnavailableProvider) UploadMedia(context.Context, []byte, string, string) (string, error) {
	return "", p.Err
}
func (p UnavailableProvider) DownloadMedia(context.Context, string) ([]byte, string, error) {
	return nil, "", p.Err
}
func (p UnavailableProvider) ParseWebhook([]byte) ([]IncomingMessage, []StatusUpdate, []ReactionUpdate, error) {
	return nil, nil, nil, p.Err
}
func (p UnavailableProvider) VerifySignature([]byte, string) bool { return false }
