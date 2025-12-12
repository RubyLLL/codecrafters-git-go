package commands

type Command struct {
	Args  []string
	Usage string
}

type CommandRunner interface {
	GetName() string
	Execute(c *Command) error
}
