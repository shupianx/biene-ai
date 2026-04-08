package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/tools"
)

// ── Message text building ─────────────────────────────────────────────────

func displayTextForInput(authorType, text string, attachments []DisplayAttachment) string {
	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		return trimmed
	}
	if len(attachments) == 0 {
		return ""
	}
	if authorType == authorTypeAgent {
		return "Shared files"
	}
	return "Uploaded files"
}

func buildInputText(authorType, authorID, authorName, text string, attachments []DisplayAttachment, meta *tools.AgentMessageMeta) string {
	var sb strings.Builder

	if authorType == authorTypeAgent {
		label := authorName
		if label == "" {
			label = authorID
		}
		fmt.Fprintf(&sb, "Message from agent %s (%s):", label, authorID)
		if meta != nil {
			if meta.ThreadID != "" {
				fmt.Fprintf(&sb, "\nThread ID: %s", meta.ThreadID)
			}
			if meta.MessageID != "" {
				fmt.Fprintf(&sb, "\nMessage ID: %s", meta.MessageID)
			}
			if meta.InReplyTo != "" {
				fmt.Fprintf(&sb, "\nIn reply to: %s", meta.InReplyTo)
			}
			if meta.RequiresReply {
				fmt.Fprintf(&sb, "\nA reply is requested. If you answer via SendToAgent back to %s, your first message will be treated as the reply and will not request another reply.", label)
			}
		}
		if strings.TrimSpace(text) != "" || len(attachments) > 0 || meta != nil {
			sb.WriteString("\n")
		}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		sb.WriteString(trimmed)
	}

	if len(attachments) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		if authorType == authorTypeAgent {
			sb.WriteString("Files delivered to your inbox:\n")
		} else {
			sb.WriteString("Files uploaded to your workspace:\n")
		}
		for _, att := range attachments {
			fmt.Fprintf(&sb, "- %s (%d bytes)\n", att.Path, att.Size)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// ── Attachment helpers ────────────────────────────────────────────────────

func cloneAttachments(in []DisplayAttachment) []DisplayAttachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]DisplayAttachment, len(in))
	copy(out, in)
	return out
}

func attachmentPaths(atts []DisplayAttachment) []string {
	out := make([]string, 0, len(atts))
	for _, att := range atts {
		out = append(out, att.Path)
	}
	return out
}

// ── File upload / copy ────────────────────────────────────────────────────

type UploadedFile struct {
	Name string
	Data []byte
}

func ReadUploadedFile(name string, r io.Reader) (UploadedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return UploadedFile{}, err
	}
	return UploadedFile{Name: name, Data: data}, nil
}

func StoreUploadedFiles(workDir, subdir string, files []UploadedFile) ([]DisplayAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}

	destDir := filepath.Join(workDir, subdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating upload directory: %w", err)
	}

	var attachments []DisplayAttachment
	for _, file := range files {
		baseName := filepath.Base(file.Name)
		if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
			return nil, fmt.Errorf("invalid file name %q", file.Name)
		}

		relativeName, absPath, err := uniqueDestPath(destDir, baseName)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(absPath, file.Data, 0o644); err != nil {
			return nil, fmt.Errorf("saving upload %q: %w", baseName, err)
		}

		attachments = append(attachments, DisplayAttachment{
			Name: baseName,
			Path: filepath.ToSlash(filepath.Join(subdir, relativeName)),
			Size: int64(len(file.Data)),
		})
	}

	return attachments, nil
}

func copyFilesBetweenWorkspaces(ctx context.Context, fromWorkDir, toWorkDir, subdir, sourceAgentID string, filePaths []string) ([]DisplayAttachment, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	destDir := filepath.Join(toWorkDir, subdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating inbox directory: %w", err)
	}

	var attachments []DisplayAttachment
	for _, requestedPath := range filePaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		sourcePath, _, err := ResolveWorkspacePath(fromWorkDir, requestedPath)
		if err != nil {
			return nil, fmt.Errorf("copying %q: %w", requestedPath, err)
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", requestedPath, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("directories are not supported: %q", requestedPath)
		}

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", requestedPath, err)
		}

		prefixedName := sourceAgentID + "-" + filepath.Base(sourcePath)
		relativeName, absPath, err := uniqueDestPath(destDir, prefixedName)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("writing inbox file %q: %w", prefixedName, err)
		}

		attachments = append(attachments, DisplayAttachment{
			Name: filepath.Base(sourcePath),
			Path: filepath.ToSlash(filepath.Join(subdir, relativeName)),
			Size: info.Size(),
		})
	}

	return attachments, nil
}

// ── Path helpers ──────────────────────────────────────────────────────────

func uniqueDestPath(destDir, baseName string) (string, string, error) {
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	for idx := 0; idx < 10_000; idx++ {
		candidate := baseName
		if idx > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, idx+1, ext)
		}
		absPath := filepath.Join(destDir, candidate)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return candidate, absPath, nil
		} else if err != nil {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("unable to allocate unique file name for %q", baseName)
}

func ResolveWorkspacePath(rootDir, requestedPath string) (string, string, error) {
	if requestedPath == "" {
		return "", "", fmt.Errorf("path is required")
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", err
	}
	target := requestedPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(rootAbs, requestedPath)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path %q escapes workspace root", requestedPath)
	}
	return targetAbs, filepath.ToSlash(rel), nil
}
