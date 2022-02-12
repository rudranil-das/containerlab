// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ixiac_one

import (
	"context"
	"fmt"
	"os/exec"
	"time"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var ixiacStatusConfig = struct {
	statusSleepDuration	time.Duration
	statusInProgressMsg string 
}{
	statusSleepDuration: time.Duration(time.Second * 5),
	statusInProgressMsg: "ls: ./.ready: No such file or directory", 
}

func init() {
	nodes.Register(nodes.NodeKindIXIACONE, func() nodes.Node {
		return new(ixiacOne)
	})
}

type ixiacOne struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (l *ixiacOne) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}

	return nil
}

func (l *ixiacOne) Config() *types.NodeConfig { return l.cfg }

func (*ixiacOne) PreDeploy(_, _, _ string) error { return nil }

func (l *ixiacOne) Deploy(ctx context.Context) error {
	_, err := l.runtime.CreateContainer(ctx, l.cfg)
	return err
}

func (l *ixiacOne) PostDeploy(ctx context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for ixia-c '%s' node", l.cfg.ShortName)
	return ixiacPostDeploy(ctx, l.runtime, l.cfg)
}

func (l *ixiacOne) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image
	return images
}

func (*ixiacOne) WithMgmtNet(*types.MgmtNet)               {}
func (l *ixiacOne) WithRuntime(r runtime.ContainerRuntime) { l.runtime = r }
func (l *ixiacOne) GetRuntime() runtime.ContainerRuntime   { return l.runtime }

func (l *ixiacOne) Delete(ctx context.Context) error {
	return l.runtime.DeleteContainer(ctx, l.Config().LongName)
}

func (*ixiacOne) SaveConfig(_ context.Context) error {
	return nil
}

// ixiacPostDeploy runs postdeploy actions which are required for ixia-c node
func ixiacPostDeploy(_ context.Context, r runtime.ContainerRuntime, node *types.NodeConfig) error {
    // TODO: replace following by goroutine
	for {
		readyCmd := "ls ./.ready" 
		bashcmd := fmt.Sprintf("docker exec %s %s", node.LongName, readyCmd)
		cmd := exec.Command("/bin/sh", "-c", bashcmd)
		//fmt.Println("---Cmd: ", cmd)
		out, err := cmd.CombinedOutput()
		if err != nil{
			msg := strings.TrimSuffix(string(out), "\n")
			if msg != ixiacStatusConfig.statusInProgressMsg {
				return err
			}
			time.Sleep(ixiacStatusConfig.statusSleepDuration)
		} else {
			break
		}
	}
	
	return nil
}
