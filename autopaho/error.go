package autopaho

import (
	"fmt"
	"sync"

	"github.com/eclipse/paho.golang/paho"
)

// errorHandler provides the onClientError callback function that will be called by the Paho library. The sole aim
// of this is to pass a single error onto the error channel (the library may send multiple errors; only the first
// will be processed).
// The callback userOnClientError will be called a maximum of one time. If userOnServerDisconnect is called, then
// userOnClientError will not be called (but there is a small chance that userOnClientError will be called followed
// by userOnServerDisconnect (if we encounter an error sending but there is a DISCONNECT in the queue).
type errorHandler struct {
	debug paho.Logger

	mu      sync.Mutex
	errChan chan error // receives connection errors

	userOnClientError      func(error)            // User provided onClientError function
	userOnServerDisconnect func(*paho.Disconnect) // User provided OnServerDisconnect function
}

// shutdown prevents any further calls from emitting a message
func (e *errorHandler) shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errChan = nil
}

// onClientError called by the paho library when an error occurs. We assume that the error is always fatal
func (e *errorHandler) onClientError(err error) {
	if e.handleError(err) && e.userOnClientError != nil {
		go e.userOnClientError(err)
	}
}

// onClientError called by the paho library when the server requests a disconnection (for example, as part of a
// clean broker shutdown). We want to begin attempting to reconnect when this occurs (and pass a detectable error
// to the user)
func (e *errorHandler) onServerDisconnect(d *paho.Disconnect) {
	e.handleError(&DisconnectError{err: fmt.Sprintf("server requested disconnect (reason: %d)", d.ReasonCode)})
	if e.userOnServerDisconnect != nil {
		go e.userOnServerDisconnect(d)
	}
}

// handleError ensures that only a single error is sent to the channel (all errors go to the users OnClientError function)
// Returns true if the error was sent to the channel (i.e. this is the first error we have seen)
func (e *errorHandler) handleError(err error) bool {
	e.mu.Lock()
	errChan := e.errChan // prevent any chance of deadlock with concurrent call to e.shutdown
	e.errChan = nil
	e.mu.Unlock()
	if errChan != nil {
		e.debug.Printf("received error: %s", err)
		errChan <- err
		return true
	} else {
		e.debug.Printf("received extra error: %s", err)
		return false
	}
}

// DisconnectError will be passed when the server requests disconnection (allows this error type to be detected)
type DisconnectError struct{ err string }

func (d *DisconnectError) Error() string {
	return d.err
}
