package machine

import "errors"

var (
	ErrBadArgs           = errors.New("bad arguments")
	ErrIllegalTransition = errors.New("illegal transition")
	ErrUnmetGuard        = errors.New("unmet guard")
	ErrRevisionConflict  = errors.New("revision conflict")
	ErrAmbiguousRun      = errors.New("ambiguous active run")
	ErrNotFound          = errors.New("not found")
	ErrCorruptState      = errors.New("corrupt state")
)
