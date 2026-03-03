package ostree

import (
	"fmt"
)

func (c *Ostree) pin(targetIndex int, unpin bool) error {
	deployments, err := c.ListDeployments()
	if err != nil {
		return err
	}

	root, err := c.Root()
	if err != nil {
		return err
	}

	var dep *Deployment
	for _, d := range deployments {
		if d.Index == targetIndex {
			dep = &d
			break
		}
	}
	if dep == nil {
		return fmt.Errorf("deployment with index %d not found", targetIndex)
	}
	if dep.Pinned && unpin {
		// good.
	} else if dep.Pinned && !unpin {
		return fmt.Errorf("deployment with index %d is already pinned", targetIndex)
	} else if !dep.Pinned && unpin {
		return fmt.Errorf("deployment with index %d is not pinned", targetIndex)
	} else if !dep.Pinned && !unpin {
		// good.
	}

	args := []string{"admin", "pin"}
	if unpin {
		args = append(args, "--unpin")
	}
	args = append(args,
		"--sysroot="+root,
		fmt.Sprintf("%d", targetIndex),
	)

	return c.ostreeRun(args...)
}

func (c *Ostree) Pin(targetIndex int) error {
	return c.pin(targetIndex, false)
}

func (c *Ostree) Unpin(targetIndex int) error {
	return c.pin(targetIndex, true)
}
