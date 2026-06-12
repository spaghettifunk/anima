package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/spaghettifunk/anima/internal/storage"
)

func ExtractUsage(expressions []PromQLExpression, now time.Time) ([]PromQLExpression, []MetricUsageEvidence, []ParseError) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	valid := make([]PromQLExpression, 0, len(expressions))
	evidence := make([]MetricUsageEvidence, 0)
	errors := make([]ParseError, 0)

	for _, expression := range expressions {
		expression.Expression = strings.TrimSpace(expression.Expression)
		if expression.Expression == "" {
			continue
		}
		if expression.DiscoveredAt.IsZero() {
			expression.DiscoveredAt = now
		} else {
			expression.DiscoveredAt = expression.DiscoveredAt.UTC()
		}
		expression.Day = storage.DayUTC(expression.Day)

		parsed, parserInput, err := parsePromQLExpression(expression.Expression)
		if err != nil {
			errors = append(errors, ParseError{Expression: expression, Err: err})
			continue
		}

		hashInput := parsed.String()
		if parserInput != expression.Expression {
			hashInput = expression.Expression
		}
		expression.ExpressionHash = ExpressionHash(hashInput)
		valid = append(valid, expression)
		evidence = append(evidence, expressionEvidence(expression, parsed)...)
	}

	return valid, evidence, errors
}

var grafanaVariableTokenRE = regexp.MustCompile(`^\$(?:\{[A-Za-z_][A-Za-z0-9_]*(?::[^}]*)?\}|[A-Za-z_][A-Za-z0-9_]*)$`)

var grafanaSubqueryRangeVariableRE = regexp.MustCompile(`\[\s*\$(?:\{[A-Za-z_][A-Za-z0-9_]*(?::[^}]*)?\}|[A-Za-z_][A-Za-z0-9_]*)\s*:\s*([^\]]+)\]`)

var grafanaRangeVariableRE = regexp.MustCompile(`\[\s*\$(?:\{[A-Za-z_][A-Za-z0-9_]*(?::[^}]*)?\}|[A-Za-z_][A-Za-z0-9_]*)\s*\]`)

var grafanaGroupingVariableRE = regexp.MustCompile(`(?i)\b(by|without|on|ignoring|group_left|group_right)\s*\(([^)]*)\)`)

const grafanaTemplatePlaceholder = "grafana_template_variable"

func parsePromQLExpression(expression string) (parser.Expr, string, error) {
	parserInput := grafanaSubqueryRangeVariableRE.ReplaceAllString(expression, "[5m:${1}]")
	parserInput = grafanaRangeVariableRE.ReplaceAllString(parserInput, "[5m]")
	parserInput = removeBareGrafanaMatcherVariables(parserInput)
	parserInput = removeGrafanaGroupingVariables(parserInput)
	parserInput = replaceUnquotedGrafanaVariables(parserInput)

	parsed, err := parser.NewParser(parser.Options{}).ParseExpr(parserInput)
	if err == nil {
		return parsed, parserInput, nil
	}

	wrappedInput := "{" + parserInput + "}"
	wrapped, wrappedErr := parser.NewParser(parser.Options{}).ParseExpr(wrappedInput)
	if wrappedErr == nil {
		return wrapped, wrappedInput, nil
	}

	return nil, parserInput, err
}

func removeGrafanaGroupingVariables(expression string) string {
	return grafanaGroupingVariableRE.ReplaceAllStringFunc(expression, func(grouping string) string {
		matches := grafanaGroupingVariableRE.FindStringSubmatch(grouping)
		if len(matches) != 3 {
			return grouping
		}

		labels := splitMatcherList(matches[2])
		kept := make([]string, 0, len(labels))
		removedVariable := false
		for _, label := range labels {
			trimmed := strings.TrimSpace(label)
			if trimmed == "" {
				continue
			}
			if grafanaVariableTokenRE.MatchString(trimmed) {
				removedVariable = true
				continue
			}
			kept = append(kept, trimmed)
		}
		if len(kept) == 0 && removedVariable {
			placeholder := grafanaTemplatePlaceholder + "_" + strings.ToLower(matches[1])
			kept = append(kept, placeholder)
		}
		return matches[1] + "(" + strings.Join(kept, ",") + ")"
	})
}

func replaceUnquotedGrafanaVariables(expression string) string {
	var out strings.Builder
	inQuote := byte(0)
	escaped := false
	for i := 0; i < len(expression); i++ {
		c := expression[i]
		if escaped {
			escaped = false
			out.WriteByte(c)
			continue
		}
		if inQuote != 0 {
			if inQuote != '`' && c == '\\' {
				escaped = true
			} else if c == inQuote {
				inQuote = 0
			}
			out.WriteByte(c)
			continue
		}
		switch c {
		case '\'', '"', '`':
			inQuote = c
			out.WriteByte(c)
		case '$':
			end, ok := grafanaVariableEnd(expression, i)
			if !ok {
				out.WriteByte(c)
				continue
			}
			if isMetricVariablePosition(expression, i, end) {
				out.WriteByte('_')
				out.WriteString(grafanaTemplatePlaceholder)
			} else {
				out.WriteByte('1')
			}
			i = end - 1
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func grafanaVariableEnd(expression string, start int) (int, bool) {
	if start+1 >= len(expression) {
		return start, false
	}
	if expression[start+1] == '{' {
		for i := start + 2; i < len(expression); i++ {
			if expression[i] == '}' {
				return i + 1, i > start+2
			}
		}
		return start, false
	}
	if !isPromQLIdentChar(expression[start+1]) {
		return start, false
	}
	end := start + 2
	for end < len(expression) && isPromQLIdentChar(expression[end]) {
		end++
	}
	return end, true
}

func isMetricVariablePosition(expression string, start, end int) bool {
	prev := previousNonSpaceByte(expression, start)
	next := nextNonSpaceByte(expression, end)
	return isPromQLIdentChar(prev) || isPromQLIdentChar(next) || next == '{' || next == '['
}

func previousNonSpaceByte(expression string, before int) byte {
	for i := before - 1; i >= 0; i-- {
		if expression[i] != ' ' && expression[i] != '\n' && expression[i] != '\r' && expression[i] != '\t' {
			return expression[i]
		}
	}
	return 0
}

func nextNonSpaceByte(expression string, after int) byte {
	for i := after; i < len(expression); i++ {
		if expression[i] != ' ' && expression[i] != '\n' && expression[i] != '\r' && expression[i] != '\t' {
			return expression[i]
		}
	}
	return 0
}

func isPromQLIdentChar(c byte) bool {
	return c == '_' || c == ':' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func removeBareGrafanaMatcherVariables(expression string) string {
	var out strings.Builder
	for i := 0; i < len(expression); i++ {
		if expression[i] != '{' {
			out.WriteByte(expression[i])
			continue
		}

		end := findMatcherCloseBrace(expression, i+1)
		if end == -1 {
			out.WriteString(expression[i:])
			break
		}

		matchers := splitMatcherList(expression[i+1 : end])
		kept := make([]string, 0, len(matchers))
		removed := false
		for _, matcher := range matchers {
			trimmed := strings.TrimSpace(matcher)
			if grafanaVariableTokenRE.MatchString(trimmed) {
				removed = true
				continue
			}
			if trimmed != "" {
				kept = append(kept, trimmed)
			}
		}

		if removed {
			out.WriteByte('{')
			out.WriteString(strings.Join(kept, ","))
			out.WriteByte('}')
		} else {
			out.WriteString(expression[i : end+1])
		}
		i = end
	}
	return out.String()
}

func findMatcherCloseBrace(expression string, start int) int {
	inQuote := byte(0)
	escaped := false
	for i := start; i < len(expression); i++ {
		c := expression[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuote != 0 {
			if inQuote != '`' && c == '\\' {
				escaped = true
				continue
			}
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inQuote = c
		case '}':
			return i
		}
	}
	return -1
}

func splitMatcherList(matchers string) []string {
	out := make([]string, 0)
	start := 0
	inQuote := byte(0)
	escaped := false
	for i := 0; i < len(matchers); i++ {
		c := matchers[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuote != 0 {
			if inQuote != '`' && c == '\\' {
				escaped = true
				continue
			}
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inQuote = c
		case ',':
			out = append(out, matchers[start:i])
			start = i + 1
		}
	}
	return append(out, matchers[start:])
}

func ExpressionHash(expression string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(expression)))
	return hex.EncodeToString(sum[:])
}

func ExtractMetricNames(query string) []string {
	metricNames, err := ExtractMetricNamesWithError(query)
	if err != nil {
		return nil
	}
	return metricNames
}

func ExtractMetricNamesWithError(query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	parsed, _, err := parsePromQLExpression(query)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	parser.Inspect(parsed, func(node parser.Node, _ []parser.Node) error {
		selector, ok := node.(*parser.VectorSelector)
		if !ok {
			return nil
		}
		for _, metricName := range metricNames(selector) {
			seen[metricName] = struct{}{}
		}
		return nil
	})

	return mapKeys(seen), nil
}

func expressionEvidence(expression PromQLExpression, parsed parser.Expr) []MetricUsageEvidence {
	groupingLabels := map[string]struct{}{}
	matcherLabelsByMetric := map[string]map[string]struct{}{}

	parser.Inspect(parsed, func(node parser.Node, _ []parser.Node) error {
		switch n := node.(type) {
		case *parser.AggregateExpr:
			addLabels(groupingLabels, n.Grouping)
		case *parser.BinaryExpr:
			if n.VectorMatching != nil {
				addLabels(groupingLabels, n.VectorMatching.MatchingLabels)
				addLabels(groupingLabels, n.VectorMatching.Include)
			}
		case *parser.VectorSelector:
			// TODO: add selector-only query correlation by matching query label matchers
			// against observed metric label/value inventory once label values are indexed.
			metricNames := metricNames(n)
			if len(metricNames) == 0 {
				return nil
			}

			matcherLabels := matcherLabels(n)
			for _, metricName := range metricNames {
				if matcherLabelsByMetric[metricName] == nil {
					matcherLabelsByMetric[metricName] = map[string]struct{}{}
				}
				addLabels(matcherLabelsByMetric[metricName], matcherLabels)
			}
		}

		return nil
	})

	out := make([]MetricUsageEvidence, 0, len(matcherLabelsByMetric))
	for metricName, matcherLabels := range matcherLabelsByMetric {
		matcher := mapKeys(matcherLabels)
		grouping := mapKeys(groupingLabels)
		used := storage.NormalizeStringKeys(append(append([]string{}, matcher...), grouping...))

		out = append(out, MetricUsageEvidence{
			Tenant:            expression.Tenant,
			Day:               expression.Day,
			MetricName:        metricName,
			SourceType:        expression.SourceType,
			SourceRef:         expression.SourceRef,
			SourceName:        expression.SourceName,
			Expression:        expression.Expression,
			ExpressionHash:    expression.ExpressionHash,
			UsedLabels:        used,
			GroupingLabels:    grouping,
			MatcherLabels:     matcher,
			IsAlert:           expression.SourceType == UsageSourceThanosAlert,
			IsRecordingRule:   expression.SourceType == UsageSourceThanosRule,
			IsDashboard:       expression.SourceType == UsageSourceGrafanaDashboard,
			IsQueryHistory:    expression.SourceType == UsageSourceQueryHistory,
			LastQueriedAt:     expression.QueriedAt,
			QueryHistoryCount: queryHistoryCount(expression),
			QueriedAt:         queriedAtList(expression),
			DiscoveredAt:      expression.DiscoveredAt,
		})
	}

	return out
}

func queryHistoryCount(expression PromQLExpression) int64 {
	if expression.SourceType == UsageSourceQueryHistory && expression.QueriedAt != nil {
		return 1
	}
	return 0
}

func queriedAtList(expression PromQLExpression) []time.Time {
	if expression.SourceType != UsageSourceQueryHistory || expression.QueriedAt == nil {
		return nil
	}
	return []time.Time{expression.QueriedAt.UTC()}
}

func metricNames(selector *parser.VectorSelector) []string {
	seen := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !strings.Contains(name, grafanaTemplatePlaceholder) {
			seen[name] = struct{}{}
		}
	}

	add(selector.Name)
	for _, matcher := range selector.LabelMatchers {
		if matcher.Name == labels.MetricName && matcher.Type == labels.MatchEqual {
			add(matcher.Value)
		}
	}

	return mapKeys(seen)
}

func matcherLabels(selector *parser.VectorSelector) []string {
	seen := map[string]struct{}{}
	for _, matcher := range selector.LabelMatchers {
		if matcher.Name == labels.MetricName || strings.Contains(matcher.Name, grafanaTemplatePlaceholder) {
			continue
		}
		seen[matcher.Name] = struct{}{}
	}
	return mapKeys(seen)
}

func addLabels(dst map[string]struct{}, labels []string) {
	for _, label := range storage.NormalizeStringKeys(labels) {
		if strings.Contains(label, grafanaTemplatePlaceholder) {
			continue
		}
		dst[label] = struct{}{}
	}
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	return storage.NormalizeStringKeys(out)
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s: %v", e.Expression.SourceRef, e.Err)
}
