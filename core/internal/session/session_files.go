package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tinte/internal/api"
	"tinte/internal/tools"
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
	// Pure image uploads read better without placeholder text — the thumbnails
	// speak for themselves.
	hasNonImage := false
	for _, a := range attachments {
		if a.Kind != "image" {
			hasNonImage = true
			break
		}
	}
	if !hasNonImage {
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
			fmt.Fprintf(&sb, "\nIf this message is asking you to do work, answer a question, make a decision, or return a result to %s, send that response back with send_to_agent instead of only writing it in the local chat. If it only shares context or files and no response is needed, you do not need to send anything back.", label)
		}
		if strings.TrimSpace(text) != "" || len(attachments) > 0 || meta != nil {
			sb.WriteString("\n")
		}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		sb.WriteString(trimmed)
	}

	// Image attachments reach the model through a visual content channel;
	// do not advertise them here as file paths for the model to read_file.
	var fileAtts []DisplayAttachment
	for _, att := range attachments {
		if att.Kind == "image" {
			continue
		}
		fileAtts = append(fileAtts, att)
	}

	if len(fileAtts) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		if authorType == authorTypeAgent {
			sb.WriteString("Files delivered to your inbox:\n")
		} else {
			sb.WriteString("Files uploaded to your workspace:\n")
		}
		for _, att := range fileAtts {
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
	Name      string
	Data      []byte
	MediaType string
}

func ReadUploadedFile(name string, r io.Reader) (UploadedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return UploadedFile{}, err
	}
	return UploadedFile{Name: name, Data: data}, nil
}

// IsImageMediaType reports whether the given MIME type is an image the UI and
// providers should treat as inline visual content.
func IsImageMediaType(mediaType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/")
}

// hydrateImageBlocks returns a copy of msgs where every ImageBlock with an
// empty Data field has been populated from disk (relative to workDir). Only
// messages containing images get fresh Content slices; others are shared
// with the input to avoid extra allocations. An image that fails to load is
// replaced with a text placeholder so a missing file never aborts a run.
func hydrateImageBlocks(msgs []api.Message, workDir string) []api.Message {
	out := make([]api.Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		needs := false
		for _, b := range m.Content {
			if img, ok := b.(api.ImageBlock); ok && len(img.Data) == 0 {
				needs = true
				break
			}
		}
		if !needs {
			continue
		}
		fresh := make([]api.ContentBlock, len(m.Content))
		for j, b := range m.Content {
			img, ok := b.(api.ImageBlock)
			if !ok || len(img.Data) > 0 {
				fresh[j] = b
				continue
			}
			absPath, err := resolveWorkspaceChild(workDir, img.Path)
			if err != nil {
				fresh[j] = api.TextBlock{Text: fmt.Sprintf("[image unavailable: %s]", img.Path)}
				continue
			}
			data, err := os.ReadFile(absPath)
			if err != nil {
				fresh[j] = api.TextBlock{Text: fmt.Sprintf("[image unavailable: %s]", img.Path)}
				continue
			}
			img.Data = data
			fresh[j] = img
		}
		out[i].Content = fresh
	}
	return out
}

// stripImageBlockData zeroes the transient Data on any ImageBlock in msgs so
// the long-lived in-memory conversation history does not retain raw image
// bytes. It is safe to call on slices returned by hydrateImageBlocks because
// those slices were freshly allocated — originating slices from before the
// run are untouched (their ImageBlocks already had empty Data).
func stripImageBlockData(msgs []api.Message) {
	for i := range msgs {
		for j, b := range msgs[i].Content {
			if img, ok := b.(api.ImageBlock); ok && len(img.Data) > 0 {
				img.Data = nil
				msgs[i].Content[j] = img
			}
		}
	}
}

// resolveWorkspaceChild joins workDir with a known-internal relative path
// (stored by us, not supplied by a tool) and validates the result stays
// inside workDir. Separate from ResolveWorkspacePath because the .tinte
// guard does not apply to internal state reads.
func resolveWorkspaceChild(workDir, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	rootAbs, err := filepath.Abs(workDir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(relPath))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", relPath)
	}
	return targetAbs, nil
}

// StoreUploadedImage persists a single image under .tinte/assets/user/ using
// an opaque timestamp-based filename (originals are discarded since nothing
// references them by name). Returns a DisplayAttachment with Kind="image".
func StoreUploadedImage(workDir string, img UploadedFile) (DisplayAttachment, error) {
	destDir := filepath.Join(workDir, UserAssetsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return DisplayAttachment{}, fmt.Errorf("creating assets directory: %w", err)
	}

	ext := extensionForMediaType(img.MediaType)
	if ext == "" {
		ext = filepath.Ext(img.Name)
	}
	filename := newAssetFilename(ext)
	absPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(absPath, img.Data, 0o644); err != nil {
		return DisplayAttachment{}, fmt.Errorf("saving image: %w", err)
	}

	displayName := strings.TrimSpace(img.Name)
	return DisplayAttachment{
		Name:      displayName,
		Path:      filepath.ToSlash(filepath.Join(UserAssetsSubdir, filename)),
		Size:      int64(len(img.Data)),
		Kind:      "image",
		MediaType: img.MediaType,
	}, nil
}

func newAssetFilename(ext string) string {
	var suffix [2]byte
	_, _ = rand.Read(suffix[:])
	stamp := time.Now().Format("2006-01-02_15-04-05")
	return fmt.Sprintf("%s_%s%s", stamp, hex.EncodeToString(suffix[:]), ext)
}

func extensionForMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}

// UserUploadSubdir is where files uploaded by the user land inside a session's
// workspace. Using a dedicated folder under inbox/ keeps user and agent
// deliveries consistent in layout.
const UserUploadSubdir = "inbox/user"

// UserAssetsSubdir holds chat-level artifacts (pasted/uploaded images) that
// belong to the conversation rather than the agent's working material.
// The agent's file tools are blocked from this prefix via
// tools.IsReservedWorkspacePath; other subdirectories of .tinte/ (notably
// skills/) remain writable so the agent can author them.
const UserAssetsSubdir = ".tinte/assets/user"

// AgentInboxSubdir builds the subdirectory where files from the given source
// agent should be stored in the receiver's workspace.
func AgentInboxSubdir(sourceAgentID string) string {
	return filepath.ToSlash(filepath.Join("inbox", sourceAgentID))
}

func StoreUploadedFiles(workDir string, files []UploadedFile) ([]DisplayAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}

	subdir := UserUploadSubdir
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

// agentCopyOutcome reports which incoming files were written, skipped,
// overwritten or renamed so the sender can surface the details to its user.
type agentCopyOutcome struct {
	Attachments []DisplayAttachment
	Skipped     []string
	Overwritten []string
	Renamed     []string
}

func copyFilesBetweenWorkspaces(
	ctx context.Context,
	fromWorkDir, toWorkDir, sourceAgentID string,
	filePaths []string,
	strategy tools.CollisionResolution,
) (agentCopyOutcome, error) {
	var outcome agentCopyOutcome
	if len(filePaths) == 0 {
		return outcome, nil
	}

	if strategy == "" {
		strategy = tools.CollisionRename
	}

	subdir := AgentInboxSubdir(sourceAgentID)
	destDir := filepath.Join(toWorkDir, subdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return outcome, fmt.Errorf("creating inbox directory: %w", err)
	}

	for _, requestedPath := range filePaths {
		select {
		case <-ctx.Done():
			return outcome, ctx.Err()
		default:
		}

		sourcePath, _, err := ResolveWorkspacePath(fromWorkDir, requestedPath)
		if err != nil {
			return outcome, fmt.Errorf("copying %q: %w", requestedPath, err)
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return outcome, fmt.Errorf("stat %q: %w", requestedPath, err)
		}
		if info.IsDir() {
			return outcome, fmt.Errorf("directories are not supported: %q", requestedPath)
		}

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return outcome, fmt.Errorf("reading %q: %w", requestedPath, err)
		}

		baseName := filepath.Base(sourcePath)
		destAbs := filepath.Join(destDir, baseName)
		_, statErr := os.Stat(destAbs)
		exists := statErr == nil

		finalName := baseName
		switch {
		case exists && strategy == tools.CollisionSkip:
			outcome.Skipped = append(outcome.Skipped, filepath.ToSlash(filepath.Join(subdir, baseName)))
			continue
		case exists && strategy == tools.CollisionOverwrite:
			outcome.Overwritten = append(outcome.Overwritten, filepath.ToSlash(filepath.Join(subdir, baseName)))
		case exists: // CollisionRename (default)
			unique, absPath, err := uniqueDestPath(destDir, baseName)
			if err != nil {
				return outcome, err
			}
			finalName = unique
			destAbs = absPath
			outcome.Renamed = append(outcome.Renamed, filepath.ToSlash(filepath.Join(subdir, finalName)))
		}

		if err := os.WriteFile(destAbs, data, 0o644); err != nil {
			return outcome, fmt.Errorf("writing inbox file %q: %w", finalName, err)
		}

		outcome.Attachments = append(outcome.Attachments, DisplayAttachment{
			Name: baseName,
			Path: filepath.ToSlash(filepath.Join(subdir, finalName)),
			Size: info.Size(),
		})
	}

	return outcome, nil
}

// detectAgentInboxCollisions reports which of the requested source files
// would clash with existing names in the target's inbox directory.
func detectAgentInboxCollisions(fromWorkDir, toWorkDir, sourceAgentID string, filePaths []string) ([]tools.FileCollision, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	subdir := AgentInboxSubdir(sourceAgentID)
	destDir := filepath.Join(toWorkDir, subdir)

	var collisions []tools.FileCollision
	for _, requestedPath := range filePaths {
		sourcePath, _, err := ResolveWorkspacePath(fromWorkDir, requestedPath)
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", requestedPath, err)
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", requestedPath, err)
		}
		if info.IsDir() {
			continue
		}
		baseName := filepath.Base(sourcePath)
		candidate := filepath.Join(destDir, baseName)
		if _, err := os.Stat(candidate); err == nil {
			collisions = append(collisions, tools.FileCollision{
				RequestedPath: requestedPath,
				TargetPath:    filepath.ToSlash(filepath.Join(subdir, baseName)),
			})
		}
	}
	return collisions, nil
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
	if tools.IsReservedWorkspacePath(rel) {
		return "", "", fmt.Errorf("path %q is reserved for session state", requestedPath)
	}
	return targetAbs, filepath.ToSlash(rel), nil
}

// ResolveSessionAssetPath resolves a path that must live under the session's
// asset directory. Unlike ResolveWorkspacePath it deliberately allows paths
// inside .tinte/ — it is used by the HTTP layer to serve user-uploaded
// images to the renderer, never by agent tools.
func ResolveSessionAssetPath(rootDir, requestedPath string) (string, error) {
	if requestedPath == "" {
		return "", fmt.Errorf("path is required")
	}
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	assetRoot := filepath.Join(rootAbs, filepath.FromSlash(UserAssetsSubdir))
	target := filepath.Join(assetRoot, filepath.FromSlash(requestedPath))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(assetRoot, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes assets directory", requestedPath)
	}
	return targetAbs, nil
}
