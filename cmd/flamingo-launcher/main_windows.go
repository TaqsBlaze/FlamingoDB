//go:build windows

// Flamingo launcher starts a local daemon and keeps its stdin open until stopped.
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const (
	mutexName          = `Local\FlamingoDBLauncher`
	eventName          = `Local\FlamingoDBStop`
	address            = "127.0.0.1:8080"
	siteURL            = "http://" + address + "/"
	errorAlreadyExists = syscall.Errno(183)
	waitObject0        = 0
	createNoWindow     = 0x08000000
)

var (
	kernel              = syscall.NewLazyDLL("kernel32.dll")
	user32              = syscall.NewLazyDLL("user32.dll")
	shell32             = syscall.NewLazyDLL("shell32.dll")
	createMutex         = kernel.NewProc("CreateMutexW")
	createEvent         = kernel.NewProc("CreateEventW")
	openEvent           = kernel.NewProc("OpenEventW")
	setEvent            = kernel.NewProc("SetEvent")
	waitForSingleObject = kernel.NewProc("WaitForSingleObject")
	closeHandle         = kernel.NewProc("CloseHandle")
	messageBox          = user32.NewProc("MessageBoxW")
	shellExecute        = shell32.NewProc("ShellExecuteW")
)

func wide(s string) *uint16 { return syscall.StringToUTF16Ptr(s) }
func alert(message string) {
	messageBox.Call(0, uintptr(unsafe.Pointer(wide(message))), uintptr(unsafe.Pointer(wide("FlamingoDB"))), 0x10)
}
func openBrowser() error {
	r, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(siteURL))), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("could not open the browser (Windows error %d)", r)
	}
	return nil
}
func stop() {
	// EVENT_MODIFY_STATE; only signal an existing launcher's event.
	h, _, _ := openEvent.Call(2, 0, uintptr(unsafe.Pointer(wide(eventName))))
	if h == 0 {
		alert("FlamingoDB is not running.")
		return
	}
	defer closeHandle.Call(h)
	setEvent.Call(h)
}
func waitReady(timeout time.Duration, exited <-chan error) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if exited != nil {
			select {
			case err := <-exited:
				return fmt.Errorf("server exited during startup: %v", err)
			default:
			}
		}
		resp, err := client.Get(siteURL)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("the server did not become ready in time")
}
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "--stop" {
		stop()
		return nil
	}
	mutex, _, last := createMutex.Call(0, 0, uintptr(unsafe.Pointer(wide(mutexName))))
	if mutex == 0 {
		return fmt.Errorf("could not create launcher mutex: %v", last)
	}
	defer closeHandle.Call(mutex)
	if last == errorAlreadyExists {
		if err := waitReady(15*time.Second, nil); err != nil {
			return err
		}
		return openBrowser()
	}
	event, _, err := createEvent.Call(0, 1, 0, uintptr(unsafe.Pointer(wide(eventName))))
	if event == 0 {
		return fmt.Errorf("could not create stop event: %v", err)
	}
	defer closeHandle.Call(event)
	// Avoid opening an unrelated service, or starting a second database on a busy port.
	for _, addr := range []string{address, "127.0.0.1:4080"} {
		listener, e := net.Listen("tcp", addr)
		if e != nil {
			return fmt.Errorf("%s is already in use. Stop the other service and try again.", addr)
		}
		listener.Close()
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return fmt.Errorf("LOCALAPPDATA is not set")
	}
	dataDir := filepath.Join(local, "FlamingoDB", "data")
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		return e
	}
	logFile, e := os.OpenFile(filepath.Join(local, "FlamingoDB", "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	defer logFile.Close()
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(executable), "flamingodbd.exe"), "-dir", dataDir, "-http", address, "-tcp", "127.0.0.1:4080", "-shutdown-stdin")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Stdout, cmd.Stderr = logFile, logFile
	stdin, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		stdin.Close()
		return fmt.Errorf("could not start the server: %w", e)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	if e = waitReady(15*time.Second, exited); e != nil {
		stdin.Close()
		return fmt.Errorf("%v. See %s", e, logFile.Name())
	}
	if e = openBrowser(); e != nil {
		stdin.Close()
		<-exited
		return e
	}
	stopped := make(chan struct{})
	go func() { waitForSingleObject.Call(event, 0xffffffff); close(stopped) }()
	select {
	case <-stopped:
		stdin.Close()
		select {
		case <-exited:
		case <-time.After(15 * time.Second):
			cmd.Process.Kill()
			<-exited
		}
	case e = <-exited:
		stdin.Close()
		return fmt.Errorf("server stopped unexpectedly: %v. See %s", e, logFile.Name())
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		alert(err.Error())
	}
}
