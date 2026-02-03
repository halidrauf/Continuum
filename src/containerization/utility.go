// Copyright (c) 2026 Khaled Abbas
//
// This source code is licensed under the Business Source License 1.1.
//
// Change Date: 4 years after the first public release of this version.
// Change License: MIT
//
// On the Change Date, this version of the code automatically converts
// to the MIT License. Prior to that date, use is subject to the
// Additional Use Grant. See the LICENSE file for details.

package containerization

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"archive/tar"
	"bytes"
	"io"

	"continuumworker/src/logging"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)


var (
	activeContainerMu sync.Mutex
	activeContainerID string
	lastUsedAt        time.Time
)

const sandboxNetworkName = "continuum_sandbox"

// ensureSandboxNetwork creates or retrieves the sandbox network for container isolation
// This network allows external internet access but we use ExtraHosts to block internal services
func EnsureSandboxNetwork(ctx context.Context, cli *client.Client) (string, error) {
	// Check if network already exists
	networks, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		logging.Log(fmt.Sprintf("failed to list networks: %w", err), slog.LevelError)
		return "", err
	}

	for _, n := range networks {
		if n.Name == sandboxNetworkName {
			return n.ID, nil
		}
	}

	// Create new sandbox network
	resp, err := cli.NetworkCreate(ctx, sandboxNetworkName, network.CreateOptions{
		Driver: "bridge",
		// Note: Internal: true would block ALL external access
		// We want external access, just not internal host access
		// So we use ExtraHosts in container config instead
	})
	if err != nil {
		logging.Log(fmt.Sprintf("failed to create sandbox network: %w", err), slog.LevelError)
		return "", err
	}

	return resp.ID, nil
}

func GetOrCreateContainer(ctx context.Context, cli *client.Client, networkID string) (string, error) {
	activeContainerMu.Lock()
	defer activeContainerMu.Unlock()

	if activeContainerID != "" {
		// Check if container is still alive
		inspect, err := cli.ContainerInspect(ctx, activeContainerID)
		if err == nil && inspect.State.Running {
			lastUsedAt = time.Now()
			//sanitize active container (erase tmp and existing files)
			execConfig := container.ExecOptions{
				User:         "root",
				AttachStdout: true,
				AttachStderr: true,
				// We just remove everything in the container home directory to be safe in case a python code leaves some files behind. /root is already inaccessible.
				Cmd: []string{"sh", "-c", `
					rm -f /script.py /payload.json
					find /tmp -mindepth 1 -delete 2>/dev/null || true
					find /var/tmp -mindepth 1 -delete 2>/dev/null || true
					find /home/sandboxuser -mindepth 1 -delete 2>/dev/null || true
				`},
			}
			exeCreate, err := cli.ContainerExecCreate(ctx, activeContainerID, execConfig)
			if err != nil {
				logging.Log(fmt.Sprintf("failed to create exec: %w", err), slog.LevelError)
				return "", err
			}
			execResp, err := cli.ContainerExecAttach(ctx, exeCreate.ID, container.ExecStartOptions{})
			if err != nil {
				logging.Log(fmt.Sprintf("failed to attach to exec: %w", err), slog.LevelError)
				return "", err
			}
			defer execResp.Close()
			return activeContainerID, nil
		}
		// If not running or error, reset and create new one
		activeContainerID = ""
	}

	imageName := os.Getenv("CONTAINER_IMAGE")
	if imageName == "" {
		imageName = "python:3.9-slim"
	}

	// Resource Limits
	memoryMBStr := os.Getenv("CONTAINER_MEMORY_MB")
	if memoryMBStr == "" {
		memoryMBStr = "512"
	}
	memoryMB, _ := strconv.ParseInt(memoryMBStr, 10, 64)

	cpuLimitStr := os.Getenv("CONTAINER_CPU_LIMIT")
	if cpuLimitStr == "" {
		cpuLimitStr = "0.5"
	}
	cpuLimit, _ := strconv.ParseFloat(cpuLimitStr, 64)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Cmd:   []string{"sleep", "infinity"}, // Keep it alive
		Tty:   false,
	}, &container.HostConfig{
		Resources: container.Resources{
			Memory:   memoryMB * 1024 * 1024,
			NanoCPUs: int64(cpuLimit * math.Pow10(9)),
		},
		CapAdd: []string{"NET_ADMIN"},
		ExtraHosts: []string{
			"host.docker.internal:127.0.0.1",
			"gateway.docker.internal:127.0.0.1",
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			sandboxNetworkName: {
				NetworkID: networkID,
			},
		},
	}, nil, "")
	if err != nil {
		logging.Log(fmt.Sprintf("failed to create container: %w", err), slog.LevelError)
		return "", err
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		logging.Log(fmt.Sprintf("failed to start container: %w", err), slog.LevelError)
		return "", err
	}

	// Move setup (iptables, user) to Exec
	setupCmd := []string{"sh", "-c", `
		apt-get update -qq && apt-get install -qq -y iptables > /dev/null 2>&1
		iptables -A OUTPUT -d 10.0.0.0/8 -j DROP 2>/dev/null || true
		iptables -A OUTPUT -d 172.16.0.0/12 -j DROP 2>/dev/null || true  
		iptables -A OUTPUT -d 192.168.0.0/16 -j DROP 2>/dev/null || true
		iptables -A OUTPUT -d 169.254.0.0/16 -j DROP 2>/dev/null || true
		useradd -m -s /bin/bash sandboxuser 2>/dev/null || true
	`}
	
	setupExec, err := cli.ContainerExecCreate(ctx, resp.ID, container.ExecOptions{
		Cmd:          setupCmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("failed to create setup exec: %w", err)
	}

	setupResp, err := cli.ContainerExecAttach(ctx, setupExec.ID, container.ExecStartOptions{})
	if err != nil {
		cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("failed to attach to setup exec: %w", err)
	}
	defer setupResp.Close()

	// Wait for setup to finish
	_, _ = io.Copy(io.Discard, setupResp.Reader)

	// Check setup exit status
	setupInspect, err := cli.ContainerExecInspect(ctx, setupExec.ID)
	if err != nil || setupInspect.ExitCode != 0 {
		cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		logging.Log(fmt.Sprintf("setup exec failed (exit %d): %v", setupInspect.ExitCode, err), slog.LevelError)
		return "", err
	}

	activeContainerID = resp.ID
	lastUsedAt = time.Now()
	logging.Log(fmt.Sprintf("New persistent container created: %s", activeContainerID[:12]), slog.LevelInfo)
	return activeContainerID, nil
}

func ExecuteTaskInDocker(ctx context.Context, cli *client.Client, code string, payload string, networkID string) (string, error) {
	containerID, err := GetOrCreateContainer(ctx, cli, networkID)
	if err != nil {
		return "", err
	}

	// Prepare TAR archive with script.py and payload.json
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// script.py
	scriptData := []byte(code)
	scriptHeader := &tar.Header{
		Name: "script.py",
		Mode: 0755,
		Size: int64(len(scriptData)),
	}
	if err := tw.WriteHeader(scriptHeader); err != nil {
		return "", err
	}
	if _, err := tw.Write(scriptData); err != nil {
		return "", err
	}

	// payload.json
	payloadData := []byte(payload)
	payloadHeader := &tar.Header{
		Name: "payload.json",
		Mode: 0644,
		Size: int64(len(payloadData)),
	}
	if err := tw.WriteHeader(payloadHeader); err != nil {
		return "", err
	}
	if _, err := tw.Write(payloadData); err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		logging.Log(fmt.Sprintf("failed to close tar writer: %w", err), slog.LevelError)
		return "", err
	}

	if err := cli.CopyToContainer(ctx, containerID, "/", &buf, container.CopyToContainerOptions{}); err != nil {
		logging.Log(fmt.Sprintf("failed to copy to container: %w", err), slog.LevelError)
		return "", err
	}

	// Fix permissions and Run as sandboxuser using Exec
	execConfig := container.ExecOptions{
		User:         "root", // Use root to chown first
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{"sh", "-c", `
			chown sandboxuser:sandboxuser /script.py /payload.json
			su sandboxuser -c "python /script.py /payload.json"
		`},
	}

	execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		logging.Log(fmt.Sprintf("failed to create exec: %w", err), slog.LevelError)
		return "", err
	}

	resp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		logging.Log(fmt.Sprintf("failed to attach to exec: %w", err), slog.LevelError)
		return "", err
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			logging.Log(fmt.Sprintf("error reading exec output: %w", err), slog.LevelError)
			return "", err
		}
	}

	// Check exec exit status
	inspect, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		logging.Log(fmt.Sprintf("failed to inspect exec: %w", err), slog.LevelError)
		return stdout.String(), err
	}
	
	if inspect.ExitCode != 0 {
		logging.Log(fmt.Sprintf("script execution error (exit %d): %s", inspect.ExitCode, stderr.String()), slog.LevelError)
		return stdout.String(), err
	}

	activeContainerMu.Lock()
	lastUsedAt = time.Now()
	activeContainerMu.Unlock()

	return stdout.String(), nil
}

func RunContainerReaper(ctx context.Context, cli *client.Client, timeout time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			activeContainerMu.Lock()
			if activeContainerID != "" && time.Since(lastUsedAt) > timeout {
				logging.Log(fmt.Sprintf("Idle timeout reached for container %s. Removing...\n", activeContainerID[:12]), slog.LevelInfo)
				id := activeContainerID
				activeContainerID = ""
				activeContainerMu.Unlock()

				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				cli.ContainerRemove(cleanupCtx, id, container.RemoveOptions{Force: true})
				cancel()
			} else {
				activeContainerMu.Unlock()
			}
		}
	}
}

func CleanupActiveContainer(ctx context.Context, cli *client.Client) {
	activeContainerMu.Lock()
	defer activeContainerMu.Unlock()

	if activeContainerID != "" {
		logging.Log(fmt.Sprintf("Cleaning up active container %s...\n", activeContainerID[:12]), slog.LevelInfo)
		cli.ContainerRemove(ctx, activeContainerID, container.RemoveOptions{Force: true})
		activeContainerID = ""
	}
}