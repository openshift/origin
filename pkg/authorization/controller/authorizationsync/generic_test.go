package authorizationsync

import (
	"fmt"

	clienttesting "k8s.io/client-go/testing"
)

func ensureSingleCreateAction(actions []clienttesting.Action) (clienttesting.CreateAction, error) {
	// ought to have one action writing status
	if len(actions) != 1 {
		return nil, fmt.Errorf("expected %v, got %v", 1, actions)
	}

	action, ok := actions[0].(clienttesting.CreateAction)
	if !ok {
		return nil, fmt.Errorf("expected %v, got %v", "create", actions[0])
	}

	return action, nil
}

func ensureSingleUpdateAction(actions []clienttesting.Action) (clienttesting.UpdateAction, error) {
	// ought to have one action writing status
	if len(actions) != 1 {
		return nil, fmt.Errorf("expected %v, got %v", 1, actions)
	}

	action, ok := actions[0].(clienttesting.UpdateAction)
	if !ok {
		return nil, fmt.Errorf("expected %v, got %v", "update", actions[0])
	}

	return action, nil
}

func ensureSingleDeleteAction(actions []clienttesting.Action) (clienttesting.DeleteAction, error) {
	// ought to have one action writing status
	if len(actions) != 1 {
		return nil, fmt.Errorf("expected %v, got %v", 1, actions)
	}

	action, ok := actions[0].(clienttesting.DeleteAction)
	if !ok {
		return nil, fmt.Errorf("expected %v, got %v", "update", actions[0])
	}

	return action, nil
}

// reactionMatch used to add reaction funcs
type reactionMatch struct {
	verb     string
	resource string
}
