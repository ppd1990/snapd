// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package ctlcmd contains the various snapctl subcommands.
package ctlcmd

import (
	"bytes"
	"fmt"
	"io"

	"github.com/snapcore/snapd/logger"
	"github.com/snapcore/snapd/overlord/hookstate"

	"github.com/jessevdk/go-flags"
)

type baseCommand struct {
	stdout io.Writer
	stderr io.Writer
	c      *hookstate.Context
}

func (c *baseCommand) setStdout(w io.Writer) {
	c.stdout = w
}

func (c *baseCommand) printf(format string, a ...interface{}) {
	if c.stdout != nil {
		fmt.Fprintf(c.stdout, format, a...)
	}
}

func (c *baseCommand) setStderr(w io.Writer) {
	c.stderr = w
}

func (c *baseCommand) errorf(format string, a ...interface{}) {
	if c.stderr != nil {
		fmt.Fprintf(c.stderr, format, a...)
	}
}

func (c *baseCommand) setContext(context *hookstate.Context) {
	c.c = context
}

func (c *baseCommand) context() *hookstate.Context {
	return c.c
}

type command interface {
	setStdout(w io.Writer)
	setStderr(w io.Writer)

	setContext(context *hookstate.Context)
	context() *hookstate.Context

	Execute(args []string) error
}

type commandInfo struct {
	shortHelp string
	longHelp  string
	generator func() command
	hidden    bool
}

var commands = make(map[string]*commandInfo)

func addCommand(name, shortHelp, longHelp string, generator func() command) *commandInfo {
	cmd := &commandInfo{
		shortHelp: shortHelp,
		longHelp:  longHelp,
		generator: generator,
	}
	commands[name] = cmd
	return cmd
}

// ForbiddenCommandError conveys that a command cannot be invoked in some context
type ForbiddenCommandError struct {
	Message string
}

func (f ForbiddenCommandError) Error() string {
	return f.Message
}

// ForbiddenCommand contains information about an attempt to use a command in a context where it is not allowed.
type ForbiddenCommand struct {
	Uid  uint32
	Name string
}

func (f *ForbiddenCommand) Execute(args []string) error {
	return &ForbiddenCommandError{Message: fmt.Sprintf("cannot use %q with uid %d, try with sudo", f.Name, f.Uid)}
}

// Run runs the requested command.
func Run(context *hookstate.Context, args []string, uid uint32) (stdout, stderr []byte, err error) {
	parser := flags.NewParser(nil, flags.PassDoubleDash|flags.HelpFlag)

	// Create stdout/stderr buffers, and make sure commands use them.
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	for name, cmdInfo := range commands {
		var data interface{}
		// commands listed here will be allowed for regular users
		// note: commands still need valid context and snaps can only access own config.
		if uid == 0 || name == "get" || name == "services" || name == "set-health" {
			cmd := cmdInfo.generator()
			cmd.setStdout(&stdoutBuffer)
			cmd.setStderr(&stderrBuffer)
			cmd.setContext(context)
			data = cmd
		} else {
			data = &ForbiddenCommand{Uid: uid, Name: name}
		}
		theCmd, err := parser.AddCommand(name, cmdInfo.shortHelp, cmdInfo.longHelp, data)
		theCmd.Hidden = cmdInfo.hidden
		if err != nil {
			logger.Panicf("cannot add command %q: %s", name, err)
		}
	}

	_, err = parser.ParseArgs(args)
	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}
