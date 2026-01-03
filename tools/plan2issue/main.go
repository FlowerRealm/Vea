package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type plan struct {
	Path      string
	Task      string
	CreatedAt string
	Overview  []string
	Steps     []planStep
}

type planStep struct {
	Number  int
	Summary string
	Details []string
}

func main() {
	inPath := flag.String("in", "", "plan markdown file path (default: newest file under ./plan)")
	outPath := flag.String("out", "issue.csv", "output csv path")
	flag.Parse()

	if err := run(*inPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	if inPath == "" {
		latest, err := newestPlanFile("plan")
		if err != nil {
			return err
		}
		inPath = latest
	}

	absIn, err := filepath.Abs(inPath)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}

	p, err := parsePlan(absIn)
	if err != nil {
		return err
	}

	if len(p.Steps) == 0 {
		return fmt.Errorf("no steps found in plan: %s", p.Path)
	}

	if err := writeIssuesCSV(outPath, p); err != nil {
		return err
	}

	return nil
}

func newestPlanFile(planDir string) (string, error) {
	entries, err := os.ReadDir(planDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("plan directory not found: %s", planDir)
		}
		return "", fmt.Errorf("read plan directory: %w", err)
	}

	candidates := make([]string, 0)
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(ent.Name()), ".md") {
			continue
		}
		candidates = append(candidates, filepath.Join(planDir, ent.Name()))
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no plan markdown files found under: %s", planDir)
	}

	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

func parsePlan(path string) (plan, error) {
	f, err := os.Open(path)
	if err != nil {
		return plan{}, fmt.Errorf("open plan: %w", err)
	}
	defer f.Close()

	lines, err := readLines(f)
	if err != nil {
		return plan{}, fmt.Errorf("read plan: %w", err)
	}

	p := plan{Path: path}

	bodyLines := lines
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end > 0 {
			meta := parseFrontmatter(lines[1:end])
			p.Task = meta["task"]
			p.CreatedAt = meta["created_at"]
			bodyLines = lines[end+1:]
		}
	}

	p.Overview = extractSection(bodyLines, "🎯 任务概述", "📋 执行计划")
	p.Steps = extractSteps(bodyLines, "📋 执行计划")

	return p, nil
}

func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	out := make([]string, 0)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseFrontmatter(lines []string) map[string]string {
	out := make(map[string]string, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}

func extractSection(lines []string, startTitle, endTitle string) []string {
	start := indexOfLineWithPrefix(lines, startTitle)
	if start < 0 {
		return nil
	}
	end := indexOfLineWithPrefix(lines, endTitle)
	if end < 0 {
		end = len(lines)
	}
	if end <= start {
		return nil
	}

	section := make([]string, 0, end-start)
	for _, raw := range lines[start+1 : end] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		section = append(section, line)
	}
	return section
}

func extractSteps(lines []string, stepsTitle string) []planStep {
	start := indexOfLineWithPrefix(lines, stepsTitle)
	if start < 0 {
		return nil
	}

	stepRe := regexp.MustCompile(`^\s*(\d+)\.\s+(.*)$`)
	steps := make([]planStep, 0)
	var current *planStep

	for i := start + 1; i < len(lines); i++ {
		raw := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "⚠️") || strings.HasPrefix(trimmed, "📎") || strings.HasPrefix(trimmed, "#") {
			break
		}

		m := stepRe.FindStringSubmatch(trimmed)
		if m == nil {
			if current == nil {
				continue
			}
			if trimmed == "" {
				continue
			}
			current.Details = append(current.Details, raw)
			continue
		}

		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		summary := strings.TrimSpace(m[2])
		if summary == "" {
			continue
		}

		if current != nil {
			steps = append(steps, *current)
		}

		current = &planStep{Number: n, Summary: summary}
	}

	if current != nil {
		steps = append(steps, *current)
	}

	return steps
}

func indexOfLineWithPrefix(lines []string, prefix string) int {
	for i, raw := range lines {
		if strings.HasPrefix(strings.TrimSpace(raw), prefix) {
			return i
		}
	}
	return -1
}

func writeIssuesCSV(outPath string, p plan) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output csv: %w", err)
	}
	defer outFile.Close()

	w := csv.NewWriter(outFile)
	w.UseCRLF = true
	if err := w.Write([]string{"title", "body"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, step := range p.Steps {
		title := issueTitle(step.Summary)
		body := issueBody(p, step)
		if err := w.Write([]string{title, body}); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func issueTitle(stepText string) string {
	stepText = strings.TrimSpace(stepText)
	short := stepText
	if before, _, ok := strings.Cut(stepText, "："); ok && strings.TrimSpace(before) != "" {
		short = strings.TrimSpace(before)
	} else if before, _, ok := strings.Cut(stepText, ":"); ok && strings.TrimSpace(before) != "" {
		short = strings.TrimSpace(before)
	}
	short = cleanIssueTitle(short)
	return "[Plan] " + short
}

func cleanIssueTitle(s string) string {
	s = strings.TrimSpace(s)
	// GitHub issue titles do not render Markdown; remove common emphasis markers.
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.TrimSpace(s)

	// Heuristic: strip a trailing "(...)" or "（...）" suffix that is usually explanatory.
	s = stripTrailingParenGroup(s, "（", "）")
	s = stripTrailingParenGroup(s, "(", ")")
	return s
}

func stripTrailingParenGroup(s, open, close string) string {
	if !strings.HasSuffix(s, close) {
		return s
	}
	idx := strings.LastIndex(s, open)
	if idx < 0 || idx >= len(s)-len(close) {
		return s
	}
	prefix := strings.TrimSpace(s[:idx])
	if prefix == "" {
		return s
	}
	return prefix
}

func issueBody(p plan, step planStep) string {
	var b strings.Builder
	b.WriteString("来源 Plan：")
	b.WriteString(p.Path)
	b.WriteString("\n")
	if p.CreatedAt != "" {
		b.WriteString("Plan 创建时间：")
		b.WriteString(p.CreatedAt)
		b.WriteString("\n")
	}
	if p.Task != "" {
		b.WriteString("总任务：")
		b.WriteString(p.Task)
		b.WriteString("\n")
	}
	if len(p.Overview) > 0 {
		b.WriteString("\n## 背景\n")
		for i, line := range p.Overview {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## 本 Issue\n")
	b.WriteString(fmt.Sprintf("%d. %s\n", step.Number, step.Summary))
	if len(step.Details) > 0 {
		for _, line := range step.Details {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}
