package ssh

import (
	"bytes"
	"text/template"
)

// ScriptRenderer renders shell script templates
type ScriptRenderer struct {
	templates map[string]*template.Template
	funcs     template.FuncMap
}

func NewScriptRenderer() *ScriptRenderer {
	return &ScriptRenderer{
		templates: make(map[string]*template.Template),
		funcs: template.FuncMap{
			"shellEscape": ShellEscape,
		},
	}
}

// Register a script template
func (sr *ScriptRenderer) Register(name, content string) error {
	tmpl, err := template.New(name).Funcs(sr.funcs).Parse(content)
	if err != nil {
		return err
	}
	sr.templates[name] = tmpl
	return nil
}

// Render a script with the given data
func (sr *ScriptRenderer) Render(name string, data interface{}) (string, error) {
	tmpl, ok := sr.templates[name]
	if !ok {
		return "", nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ShellEscape escapes a string for safe use in shell commands
func ShellEscape(s string) string {
	// Single-quote escape: replace ' with '\''
	var buf bytes.Buffer
	buf.WriteByte('\'')
	for _, c := range s {
		if c == '\'' {
			buf.WriteString("'\\''")
		} else {
			buf.WriteRune(c)
		}
	}
	buf.WriteByte('\'')
	return buf.String()
}

// Common script templates
const (
	ScriptCreateUser = `
set -e
export DEBIAN_FRONTEND=noninteractive
useradd -m -s /bin/bash {{shellEscape .Username}}
echo "{{shellEscape .Username}}:{{shellEscape .Password}}" | chpasswd
usermod -aG sudo {{shellEscape .Username}}
echo "{{shellEscape .Username}} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
mkdir -p /home/{{shellEscape .Username}}/.ssh
echo {{shellEscape .PublicKey}} >> /home/{{shellEscape .Username}}/.ssh/authorized_keys
chown -R {{shellEscape .Username}}:{{shellEscape .Username}} /home/{{shellEscape .Username}}/.ssh
chmod 700 /home/{{shellEscape .Username}}/.ssh
chmod 600 /home/{{shellEscape .Username}}/.ssh/authorized_keys
`

	ScriptInstallDeps = `
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl wget git unzip zip build-essential
`

	ScriptSystemdRestart = `
set -e
systemctl restart {{shellEscape .Service}}
systemctl is-active {{shellEscape .Service}}
`

	ScriptWriteFile = `
set -e
cat > {{shellEscape .Path}} << 'BOTGINX_EOF'
{{.Content}}
BOTGINX_EOF
chmod {{.Mode}} {{shellEscape .Path}}
{{if .Owner}}chown {{shellEscape .Owner}}:{{shellEscape .Owner}} {{shellEscape .Path}}{{end}}
`

	ScriptRunScript = `
set -e
set -o pipefail
cd {{shellEscape .WorkDir}}
{{.Script}}
`
)

// DefaultRenderer with common scripts registered
func DefaultRenderer() *ScriptRenderer {
	sr := NewScriptRenderer()
	sr.Register("create-user", ScriptCreateUser)
	sr.Register("install-deps", ScriptInstallDeps)
	sr.Register("systemd-restart", ScriptSystemdRestart)
	sr.Register("write-file", ScriptWriteFile)
	sr.Register("run-script", ScriptRunScript)
	return sr
}
