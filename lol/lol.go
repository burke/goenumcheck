package self

type demoEnum int

const (
	aDemo demoEnum = iota
	bDemo
)

func neato(t demoEnum) {
	switch t {
	case aDemo:
	}
}
