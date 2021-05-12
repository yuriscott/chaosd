// Copyright 2020 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package chaosd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	"github.com/pingcap/errors"
	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/chaos-mesh/chaosd/pkg/core"
)

const ruleTemplate = `
RULE {{.Name}}
CLASS {{.Class}}
METHOD {{.Method}}
AT ENTRY
IF true
DO 
	{{.Do}};
ENDRULE
`

const stressRuleTemplate = `
RULE {{.Name}}
STRESS {{.StressType}}
{{.StressValueName}} {{.StressValue}}
ENDRULE
`

const gcRuleTemplate = `
RULE {{.Name}}
GC
ENDRULE
`

type jvmAttack struct{}

var JVMAttack AttackType = jvmAttack{}

const bmInstallCommand = "bminstall.sh -b -Dorg.jboss.byteman.transform.all -Dorg.jboss.byteman.verbose -p %d %d"
const bmSubmitCommand = "bmsubmit.sh -p %d -%s %s"

func (j jvmAttack) Attack(options core.AttackConfig, env Environment) (err error) {
	attack := options.(*core.JVMCommand)

	if attack.Type == core.JVMInstallType {
		return j.install(attack)
	} else if attack.Type == core.JVMSubmitType {
		return j.submit(attack)
	}

	return errors.Errorf("attack type %s not supported", attack.Type)
}

func (j jvmAttack) install(attack *core.JVMCommand) error {
	var err error

	bmInstallCmd := fmt.Sprintf(bmInstallCommand, attack.Port, attack.Pid)
	cmd := exec.Command("bash", "-c", bmInstallCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(string(output), zap.Error(err))
		return err
	}

	log.Info(string(output))
	return err
}

func (j jvmAttack) submit(attack *core.JVMCommand) error {
	var err error

	if len(attack.Do) == 0 {
		switch attack.Action {
		case core.JVMLatencyAction:
			attack.Do = fmt.Sprintf("Thread.sleep(%s)", attack.LatencyDuration)
		case core.JVMExceptionAction:
			attack.Do = fmt.Sprintf("throw new %s", attack.ThrowException)
		case core.JVMReturnAction:
			attack.Do = fmt.Sprintf("return %s", attack.ReturnValue)
		case core.JVMStressAction:
			if attack.CPUCount > 0 {
				attack.StressType = "CPU"
				attack.StressValueName = "CPUCOUNT"
				attack.StressValue = attack.CPUCount
			} else {
				attack.StressType = "MEMORY"
				attack.StressValueName = "MEMORYSIZE"
				attack.StressValue = attack.MemorySize
			}

		}
	}

	buf := new(bytes.Buffer)

	var t *template.Template
	switch attack.Action {
	case core.JVMStressAction:
		t = template.Must(template.New("byteman rule").Parse(stressRuleTemplate))
	case core.JVMExceptionAction, core.JVMLatencyAction, core.JVMReturnAction:
		t = template.Must(template.New("byteman rule").Parse(ruleTemplate))
	case core.JVMGCAction:
		t = template.Must(template.New("byteman rule").Parse(gcRuleTemplate))
	default:
		return errors.Errorf("jvm action %s not supported", attack.Action)
	}

	if t == nil {
		return errors.Errorf("parse byeman rule template failed")
	}

	err = t.Execute(buf, attack)
	if err != nil {
		log.Error("executing template", zap.Error(err))
		return err
	}

	log.Info("byteman rule", zap.String("rule", buf.String()))

	tmpfile, err := ioutil.TempFile("", "rule.btm")
	if err != nil {
		return err
	}

	log.Info("create btm file", zap.String("file", tmpfile.Name()))

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(buf.Bytes()); err != nil {
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}

	bmSubmitCmd := fmt.Sprintf(bmSubmitCommand, attack.Port, "l", tmpfile.Name())
	cmd := exec.Command("bash", "-c", bmSubmitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(string(output), zap.Error(err))
		return err
	}

	log.Info(string(output))
	return nil
}

func (j jvmAttack) Recover(exp core.Experiment, env Environment) error {
	attack := &core.JVMCommand{}
	if err := json.Unmarshal([]byte(exp.RecoverCommand), attack); err != nil {
		return err
	}

	// Create a new template and parse the letter into it.
	t := template.Must(template.New("byteman rule").Parse(ruleTemplate))

	buf := new(bytes.Buffer)
	err := t.Execute(buf, attack)
	if err != nil {
		log.Error("executing template", zap.Error(err))
		return err
	}

	tmpfile, err := ioutil.TempFile("", "rule.btm")
	if err != nil {
		return err
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(buf.Bytes()); err != nil {
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}

	log.Info("create btm file", zap.String("file", tmpfile.Name()))

	bmSubmitCmd := fmt.Sprintf(bmSubmitCommand, attack.Port, "u", tmpfile.Name())
	cmd := exec.Command("bash", "-c", bmSubmitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(string(output), zap.Error(err))
		return err
	}

	log.Info(string(output))

	return nil
}
