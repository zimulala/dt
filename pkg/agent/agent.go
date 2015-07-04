package agent

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/ngaut/log"
	"github.com/pingcap/dt/pkg/util"
)

const (
	instanceStateUninitialized = "uninitialized"
	instanceStateStarted       = "started"
	instanceStateStopped       = "stopped"
	instanceStatePaused        = "paused"
	instanceStateContinue      = "continue"

	pauseInstanceCmd       = "kill -STOP"
	continueInstanceCmd    = "kill -CONT"
	backupInstanceDataCmd  = "cp -r"
	cleanUpInstanceDataCmd = "rm -r"

	registerIntervalTime = 50 //msec
)

type Agent struct {
	Ip       string
	Addr     string
	CtrlAddr string

	cmd           *exec.Cmd
	logfile       *os.File
	l             net.Listener
	instanDir     string
	instancePid   int
	instanceState string
}

func NewAgent(cfg *AgentCfg) (*Agent, error) {
	if _, err := os.Stat(cfg.DataDir); err != nil {
		return nil, err
	}

	f, err := os.Create(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	return &Agent{
		Ip:            cfg.Ip,
		Addr:          fmt.Sprintf("%s:%s", cfg.Ip, cfg.Port),
		CtrlAddr:      cfg.CtrlAddr,
		instanceState: instanceStateUninitialized,
		cmd:           exec.Command(cfg.DataDir),
		logfile:       f}, nil
}

func (a *Agent) Register() error {
	log.Debug("start: register")
	agentAttr := make(url.Values)
	agentAttr.Set("addr", a.Addr)

	return util.HttpCall(util.ApiUrl(a.CtrlAddr, "api/agent/register", agentAttr.Encode()), "POST", nil)
}

func (a *Agent) Start() error {
	for {
		if err := a.Register(); err != nil {
			log.Warning("register failed, err:", err)
			time.Sleep(registerIntervalTime * time.Millisecond)
		}
		break
	}

	return runHttpServer(a)
}

func (a *Agent) execCmd(cmd *exec.Cmd, args ...string) error {
	cmd = exec.Command(cmd.Path, args...)
	cmd.Stdout = a.logfile
	cmd.Stderr = a.logfile

	return cmd.Run()
}

func (a *Agent) StartInstance(args ...string) error {
	log.Debug("start: startInstance, agent")
	if err := a.execCmd(a.cmd, args...); err != nil {
		return err
	}

	a.instanceState = instanceStateStarted
	a.instancePid = a.cmd.Process.Pid

	return nil
}

func (a *Agent) RestarInstance(args ...string) error {
	if err := a.StopInstance(); err != nil {
		return err
	}

	return a.StartInstance(args...)
}

func (a *Agent) PauseInstance() error {
	if a.instanceState != instanceStateStarted {
		return nil
	}

	arg := fmt.Sprintf(pauseInstanceCmd+" %d", a.instancePid)
	cmd := exec.Command("sh")
	if err := a.execCmd(cmd, "-c", arg); err != nil {
		return err
	}
	a.instanceState = instanceStatePaused

	return nil
}

func (a *Agent) ContinuePauseInstance() error {
	if a.instanceState != instanceStatePaused {
		return nil
	}

	arg := fmt.Sprintf(continueInstanceCmd+" %d", a.instancePid)
	cmd := exec.Command("sh")
	if err := a.execCmd(cmd, "-c", arg); err != nil {
		return err
	}
	a.instanceState = instanceStateStarted

	return nil

}

func (a *Agent) StopInstance() error {
	if a.instanceState != instanceStateStarted {
		return nil
	}

	if err := a.cmd.Process.Kill(); err != nil {
		return err
	}
	if _, err := a.cmd.Process.Wait(); err != nil {
		return err
	}

	a.instanceState = instanceStateStopped

	return nil
}

func (a *Agent) BackupInstanceData(args ...string) error {
	arg := fmt.Sprintf(backupInstanceDataCmd+" %s", a.instanDir)
	cmd := exec.Command("sh")

	return a.execCmd(cmd, "-c", arg)
}

func (a *Agent) CleanUpInstanceData(args ...string) error {
	// TODO: clean up intance internal state
	arg := fmt.Sprintf(cleanUpInstanceDataCmd+" %s", a.instanDir)
	cmd := exec.Command("sh")

	return a.execCmd(cmd, "-c", arg)
}

//  TODO: implement
func (a *Agent) Shutdown() error {
	panic("Shutdown, hasn't implement")

	return nil
}

//  TODO: ports may be more than one
func (a *Agent) DropPortInstance(port string) error {
	return DropPort(port)
}

func (a *Agent) RecoverPortInstance(port string) error {
	return RecoverPort(port)
}
