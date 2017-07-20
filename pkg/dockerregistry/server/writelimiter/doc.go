// Package writelimiter provides middleware that limits the number of concurrent write requests.
// Requests over the limit don't fail, but are queued and processed later.
package writelimiter
