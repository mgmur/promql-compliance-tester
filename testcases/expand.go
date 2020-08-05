package testcases

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/promlabs/promql-compliance-tester/comparer"
	"github.com/promlabs/promql-compliance-tester/config"
)

var testVariantArgs = map[string][]string{
	"range":  {"1s", "15s", "1m", "5m", "15m", "1h"},
	"offset": {"1m", "5m", "10m"},
	// TODO: Add "group" aggregator and new duration formats, but it is so new that vendor implementations need time to catch up first.
	"simpleAggrOp": {"sum", "avg", "max", "min", "count", "stddev", "stdvar"},
	"topBottomOp":  {"topk", "bottomk"},
	"quantile": {
		"-0.5",
		"0.1",
		"0.5",
		"0.75",
		"0.95",
		"0.90",
		"0.99",
		"1",
		"1.5",
	},
	"arithBinOp":           {"+", "-", "*", "/", "%", "^"},
	"compBinOp":            {"==", "!=", "<", ">", "<=", ">="},
	"binOp":                {"+", "-", "*", "/", "%", "^", "==", "!=", "<", ">", "<=", ">="},
	"simpleMathFunc":       {"abs", "ceil", "floor", "exp", "sqrt", "ln", "log2", "log10", "round"},
	"extrapolatedRateFunc": {"delta", "rate", "increase"},
	"clampFunc":            {"clamp_min", "clamp_max"},
	"instantRateFunc":      {"idelta", "irate"},
	"dateFunc":             {"day_of_month", "day_of_week", "days_in_month", "hour", "minute", "month", "year"},
	"smoothingFactor":      {"0.1", "0.5", "0.8"},
	"trendFactor":          {"0.1", "0.5", "0.8"},
}

// tprintf replaces template arguments in a string with their instantiations from the provided map.
func tprintf(tmpl string, data map[string]string) string {
	t := template.Must(template.New("Query").Parse(tmpl))
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

// getVariants returns every possible combinations (variants) of a template query.
func getVariants(query string, remainingVariantArgs []string, args map[string]string) []string {
	// Either this Query had no variants defined to begin with or they have
	// been fully filled out in "args" from recursive parent calls.
	if len(remainingVariantArgs) == 0 {
		return []string{tprintf(query, args)}
	}

	// Recursively iterate through the values for each variant arg dimension,
	// selecting one dimension (arg) to vary per recursion level and let the
	// other recursion levels iterate through the remaining dimensions until
	// all args are defined.
	var queries []string
	vArg := remainingVariantArgs[0]
	filteredVArgs := make([]string, 0, len(remainingVariantArgs)-1)
	for _, va := range remainingVariantArgs {
		if va != vArg {
			filteredVArgs = append(filteredVArgs, va)
		}
	}

	vals := testVariantArgs[vArg]
	if len(vals) == 0 {
		panic(fmt.Errorf("unknown variant arg %q", vArg))
	}
	for _, variantVal := range vals {
		args[vArg] = variantVal
		qs := getVariants(query, filteredVArgs, args)
		queries = append(queries, qs...)
	}
	return queries
}

func applyQueryTweaks(tc *comparer.TestCase, tweaks []*config.QueryTweak) *comparer.TestCase {
	resTC := *tc
	for _, t := range tweaks {
		if d := time.Duration(t.TruncateTimestampsToMS) * time.Millisecond; d != 0 {
			resTC.Start = resTC.Start.Truncate(d)
			resTC.End = resTC.End.Truncate(d)
		}
		if t.AlignTimestampsToStep {
			resTC.Start = resTC.Start.Truncate(resTC.Resolution)
			resTC.End = resTC.End.Truncate(resTC.Resolution)
		}
	}
	return &resTC
}

// ExpandTestCases returns the fully expanded test cases for a given set of templates test cases.
func ExpandTestCases(cases []*config.TestCase, tweaks []*config.QueryTweak, start, end time.Time, resolution time.Duration) []*comparer.TestCase {
	tcs := make([]*comparer.TestCase, 0)
	for _, q := range cases {
		vs := getVariants(q.Query, q.VariantArgs, make(map[string]string))
		for _, v := range vs {
			tc := &comparer.TestCase{
				Query:          v,
				SkipComparison: q.SkipComparison,
				ShouldFail:     q.ShouldFail,
				Start:          start,
				End:            end,
				Resolution:     resolution,
			}

			tcs = append(tcs, applyQueryTweaks(tc, tweaks))
		}
	}
	return tcs
}