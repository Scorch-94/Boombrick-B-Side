package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The firmwares' launch scripts are tested here, beside the code that
// stages what they apply.

// launchScript is a firmware's launch script: its file in build/, and its
// name in the app's folder.
type launchScript struct{ name, src, file string }

// launchers are the firmwares' launch scripts.
var launchers = []launchScript{
	{"NextUI", filepath.Join("..", "..", "build", "pak", "launch.sh"), "launch.sh"},
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// installed is the app as a firmware holds it: l's script, and a program
// that says which it is, with its folders.
func installed(t *testing.T, l launchScript) string {
	t.Helper()
	return installedIn(t, t.TempDir(), l)
}

// installedIn is installed, in the folder dir.
func installedIn(t *testing.T, dir string, l launchScript) string {
	t.Helper()
	body, err := os.ReadFile(l.src)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, l.file), string(body), 0o755)
	write(t, filepath.Join(dir, "boombrick"), "#!/bin/sh\necho old > ran\n", 0o755)
	write(t, filepath.Join(dir, "bin", "qjs"), "old qjs", 0o755)
	write(t, filepath.Join(dir, "lib", "libold.so"), "old", 0o644)
	return dir
}

// launch runs the script at path as its firmware does, with stand-ins for
// the firmware's programs, and returns the folder it uses for /tmp.
func launch(t *testing.T, path string) string {
	t.Helper()
	cmd, tmp := launcher(t, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", filepath.Base(path), err, out)
	}
	return tmp
}

// launcher is the command launch runs, and the folder it uses for /tmp and,
// as Knulli's, /var/run. The stand-in poweroff, given by path as busybox's
// sh would run its own before one on PATH, leaves powered-off there.
func launcher(t *testing.T, path string) (*exec.Cmd, string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	stubs, tmp := t.TempDir(), t.TempDir()
	write(t, filepath.Join(stubs, "syncsettings.elf"), "#!/bin/sh\n", 0o755)
	write(t, filepath.Join(stubs, "poweroff"), "#!/bin/sh\ntouch \"$BOOMBRICK_TMP/powered-off\"\n", 0o755)
	cmd := exec.Command(sh, path)
	cmd.Env = append(os.Environ(), "LOGS_PATH="+t.TempDir(), "BOOMBRICK_TMP="+tmp, "BOOMBRICK_RUN="+tmp,
		"BOOMBRICK_POWEROFF="+filepath.Join(stubs, "poweroff"),
		"PATH="+stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
	return cmd, tmp
}

func TestLaunchAppliesAStagedUpdate(t *testing.T) {
	for _, l := range launchers {
		t.Run(l.name, func(t *testing.T) {
			dir := installed(t, l)
			staged := filepath.Join(dir, ".update", "pak")
			write(t, filepath.Join(staged, l.file), read(t, filepath.Join(dir, l.file)), 0o755)
			write(t, filepath.Join(staged, "boombrick"), "#!/bin/sh\necho new > ran\n", 0o755)
			write(t, filepath.Join(staged, "bin", "qjs"), "new qjs", 0o755)
			write(t, filepath.Join(staged, "lib", "libnew.so"), "new", 0o644)
			write(t, filepath.Join(dir, ".update", "ready"), "0.5.0\n", 0o644)
			launch(t, filepath.Join(dir, l.file))
			if got := read(t, filepath.Join(dir, "ran")); got != "new\n" {
				t.Fatalf("the program that ran said %q", got)
			}
			if read(t, filepath.Join(dir, "bin", "qjs")) != "new qjs" || read(t, filepath.Join(dir, "lib", "libnew.so")) != "new" {
				t.Fatal("the new files weren't moved in")
			}
			if read(t, filepath.Join(dir, "lib", "libold.so")) != "old" {
				t.Fatal("a file the update doesn't have went")
			}
			if _, err := os.Stat(filepath.Join(dir, ".update")); !os.IsNotExist(err) {
				t.Fatal(".update is still there")
			}
		})
	}
}

func TestLaunchFinishesAnUpdateCutShort(t *testing.T) {
	for _, l := range launchers {
		t.Run(l.name, func(t *testing.T) {
			dir := installed(t, l)
			// The program was moved in before the power went; qjs wasn't.
			write(t, filepath.Join(dir, "boombrick"), "#!/bin/sh\necho new > ran\n", 0o755)
			write(t, filepath.Join(dir, ".update", "pak", "bin", "qjs"), "new qjs", 0o755)
			write(t, filepath.Join(dir, ".update", "ready"), "0.5.0\n", 0o644)
			launch(t, filepath.Join(dir, l.file))
			if read(t, filepath.Join(dir, "ran")) != "new\n" || read(t, filepath.Join(dir, "bin", "qjs")) != "new qjs" {
				t.Fatal("the update wasn't finished")
			}
			if _, err := os.Stat(filepath.Join(dir, ".update")); !os.IsNotExist(err) {
				t.Fatal(".update is still there")
			}
		})
	}
}

func TestLaunchDeletesAnIncompleteStage(t *testing.T) {
	for _, l := range launchers {
		t.Run(l.name, func(t *testing.T) {
			dir := installed(t, l)
			write(t, filepath.Join(dir, ".update", "pak", "boombrick"), "#!/bin/sh\necho new > ran\n", 0o755)
			launch(t, filepath.Join(dir, l.file))
			if got := read(t, filepath.Join(dir, "ran")); got != "old\n" {
				t.Fatalf("the program that ran said %q", got)
			}
			if _, err := os.Stat(filepath.Join(dir, ".update")); !os.IsNotExist(err) {
				t.Fatal("the incomplete stage is still there")
			}
		})
	}
}
