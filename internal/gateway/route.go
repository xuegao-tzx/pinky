package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/DIYgod/pinky/internal/driver"
)

// OpenURI opens a deep link on a phone. id is an Android package or an
// iOS bundle id, or empty. The phone is named by id, name or a unique
// prefix, as a tool's device argument. This does not ask for approval.
func (g *Gateway) OpenURI(ctx context.Context, deviceRef, uri, id string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("open route: empty uri")
	}
	d, err := g.deviceByRef(strings.TrimSpace(deviceRef))
	if err != nil {
		return err
	}
	dev, _ := g.screenOf(d)
	if dev == nil {
		return fmt.Errorf("open route: %s is not open", d.name())
	}
	if u, ok := dev.(driver.URIOpener); ok {
		return u.OpenURI(ctx, uri, strings.TrimSpace(id))
	}
	opener, ok := dev.(driver.LinkOpener)
	if !ok {
		return fmt.Errorf("open route: %s cannot open a link: %w", d.name(), driver.ErrUnsupported)
	}
	app := driver.App{ID: strings.TrimSpace(id), Name: strings.TrimSpace(id)}
	if app.ID == "" {
		app, err = opener.LinkApp(ctx, uri)
		if err != nil {
			return err
		}
	}
	return opener.OpenLink(ctx, uri, app)
}
