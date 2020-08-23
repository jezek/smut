package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"go.i3wm.org/i3/v4"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

func main() {
	// i3 uses standart log that writes to Stderr. I don't want i3 package to write to Stderr.
	log.SetOutput(ioutil.Discard)

	// Tell i3 package to use sway.
	i3UseSway()

	unfocusedOpacity := 0.7

	lastFocusedNode, lastFocusedWorkspace, err := getFocusedNodeAndWorkspace()
	if err != nil {
		logger.Fatalf("Error getting focused node & workspace: %v", err)
	}

	setCriteriaOpacity("title=\".*\"", unfocusedOpacity)
	if lastFocusedNode != nil {
		setNodeOpacity(*lastFocusedNode, 1)
	}

	recv := i3.Subscribe(i3.WindowEventType)
	for recv.Next() {
		switch ev := recv.Event().(type) {
		case *i3.WindowEvent:
			logger.Printf("Got WindowEvent: %v(%d) -  %s", ev.Change, ev.Container.ID, ev.Container.Name)
			if ev.Change == "focus" {
				if lastFocusedNode == nil || lastFocusedNode.ID != ev.Container.ID {
					setNodeOpacity(ev.Container, 1)

					focusedWorkspace, err := getNodeWorkspace(&ev.Container)
					if err != nil {
						logger.Printf("Error getting node (ID: %d) workspace: %v", ev.Container.ID, err)
						break
					}

					if lastFocusedNode != nil && focusedWorkspace != nil && lastFocusedWorkspace.ID == focusedWorkspace.ID {
						setNodeOpacity(*lastFocusedNode, unfocusedOpacity)
					}

					lastFocusedNode = &ev.Container
					lastFocusedWorkspace = focusedWorkspace
				}
			}
		default:
			logger.Printf("Unrecognized event type: %#v", ev)
		}
	}
	//TODO Catch signals & shutdown gracefully.
	setCriteriaOpacity("title=\".*\"", 1)
	//TODO If focusing using swaymsg to anotther workspace to earley unfocused window, there can be more fully opaque windows.
}

// Overrides i3 to work with sway.
func i3UseSway() {
	i3.SocketPathHook = func() (string, error) {
		out, err := exec.Command("sway", "--get-socketpath").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("getting sway socketpath: %v (output: %s)", err, out)
		}
		return string(out), nil
	}

	i3.IsRunningHook = func() bool {
		out, err := exec.Command("pgrep", "-c", "sway\\$").CombinedOutput()
		if err != nil {
			logger.Printf("sway running: %v (output: %s)", err, out)
		}
		return bytes.Compare(out, []byte("1")) == 0
	}
}

// Sets opacity for node.
func setNodeOpacity(node i3.Node, opacity float64) error {
	return setCriteriaOpacity("con_id=\""+strconv.FormatInt(int64(node.ID), 10)+"\"", opacity)
}

// Sets opacity for criteria.
func setCriteriaOpacity(criteria string, opacity float64) error {
	cmd := "[" + criteria + "] opacity set " + strconv.FormatFloat(opacity, 'f', 2, 64)
	res, err := i3.RunCommand(cmd)
	if err != nil && !i3.IsUnsuccessful(err) {
		logger.Printf("Error running command \"%s\": %v", cmd, err)
		return err
	}
	logger.Printf("Result of \"%s\": %v\n", cmd, res)
	return nil
}

// Gets current focused node with workspace node.
func getFocusedNodeAndWorkspace() (*i3.Node, *i3.Node, error) {
	tree, err := i3.GetTree()
	if err != nil {
		return nil, nil, err
	}
	focusedWorkspace := (*i3.Node)(nil)
	focusedNode := tree.Root.FindFocused(func(node *i3.Node) bool {
		if node.Type == i3.WorkspaceNode {
			focusedWorkspace = node
		}
		return node.Focused
	})
	return focusedNode, focusedWorkspace, nil
}

// Gets node workspace.
func getNodeWorkspace(node *i3.Node) (*i3.Node, error) {
	tree, err := i3.GetTree()
	if err != nil {
		return nil, err
	}
	workspace := (*i3.Node)(nil)
	fn := tree.Root.FindChild(func(n *i3.Node) bool {
		if n.Type == i3.WorkspaceNode {
			workspace = n
		}
		return n.ID == node.ID
	})
	if fn == nil {
		return nil, nil
	}
	return workspace, nil
}
