package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"gitee.com/IKEJAY-code/uclash/internal/mihomoapi"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Inspect and switch nodes of the running core",
	}
	cmd.AddCommand(newNodeLsCmd(), newNodeUseCmd(), newNodeTestCmd())
	return cmd
}

func isSelector(t string) bool { return strings.EqualFold(t, "Selector") }

func newNodeLsCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "ls [group]",
		Short: "List selectable groups, or the nodes inside one group",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			client, ok := a.Client()
			if !ok {
				return fmt.Errorf("core is not running (start it with `uclash start`)")
			}
			ps, err := client.GetProxies(cmd.Context())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(args) == 1 {
				g, ok := ps.Proxies[args[0]]
				if !ok {
					return fmt.Errorf("group %q not found (see `uclash node ls`)", args[0])
				}
				return printGroupNodes(out, args[0], &g, ps)
			}
			var names []string
			for name, p := range ps.Proxies {
				if isSelector(p.Type) && (all || name != "GLOBAL") {
					names = append(names, name)
				}
			}
			if len(names) == 0 {
				return fmt.Errorf("no selectable groups in the running config")
			}
			sort.Strings(names)
			fmt.Fprintln(out, "selectable groups:")
			w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			for _, n := range names {
				p := ps.Proxies[n]
				fmt.Fprintf(w, "  %s\t%d nodes\tcurrent: %s\n", n, len(p.All), orDash(p.Now))
			}
			w.Flush()
			fmt.Fprintln(out)
			fmt.Fprintln(out, "details: uclash node ls <group>       switch: uclash node use [group] <name|index>")
			fmt.Fprintln(out, "latency: uclash node test [group]")
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include the GLOBAL group")
	return cmd
}

func printGroupNodes(out io.Writer, name string, g *mihomoapi.Proxy, ps *mihomoapi.Proxies) error {
	w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	fmt.Fprintf(w, "%s (%s, current: %s)\n", name, g.Type, orDash(g.Now))
	for i, n := range g.All {
		marker := " "
		if n == g.Now {
			marker = "*"
		}
		fmt.Fprintf(w, "%s %d. %s\t%s\n", marker, i+1, n, delayString(ps, n))
	}
	return w.Flush()
}

func delayString(ps *mihomoapi.Proxies, name string) string {
	p, ok := ps.Proxies[name]
	if !ok || len(p.History) == 0 {
		return ""
	}
	d := p.History[0].Delay
	if d <= 0 {
		return "timeout"
	}
	return fmt.Sprintf("%d ms", d)
}

func newNodeUseCmd() *cobra.Command {
	var groupFlag string
	cmd := &cobra.Command{
		Use:   "use [group] <node|index>",
		Short: "Switch the active node in a select group",
		Long: `Switch the active node.

  uclash node use <node>            # auto-picks the group containing it (prefers PROXY)
  uclash node use <group> <node>    # explicit group
  uclash node use <group> <index>   # index from ` + "`uclash node ls <group>`" + ` (1-based)`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			client, ok := a.Client()
			if !ok {
				return fmt.Errorf("core is not running (start it with `uclash start`)")
			}
			ps, err := client.GetProxies(cmd.Context())
			if err != nil {
				return err
			}
			var group, target string
			if len(args) == 2 {
				group, target = args[0], args[1]
			} else {
				target = args[0]
			}
			if groupFlag != "" {
				group = groupFlag
			}
			if group == "" {
				g, gerr := pickGroup(ps, target)
				if gerr != nil {
					return gerr
				}
				group = g
			}
			g, ok := ps.Proxies[group]
			if !ok {
				return fmt.Errorf("group %q not found (see `uclash node ls`)", group)
			}
			if !isSelector(g.Type) {
				return fmt.Errorf("group %q has type %s and cannot be switched (only select groups can)", group, g.Type)
			}
			node, err := resolveNode(g, target)
			if err != nil {
				return err
			}
			if err := client.Select(cmd.Context(), group, node); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n", group, node)
			return nil
		},
	}
	cmd.Flags().StringVar(&groupFlag, "group", "", "group name (skips auto-detection)")
	return cmd
}

func resolveNode(g mihomoapi.Proxy, target string) (string, error) {
	if n, err := strconv.Atoi(target); err == nil {
		if n < 1 || n > len(g.All) {
			return "", fmt.Errorf("index %d out of range (group %q has %d nodes)", n, g.Name, len(g.All))
		}
		return g.All[n-1], nil
	}
	for _, n := range g.All {
		if n == target {
			return n, nil
		}
	}
	for _, n := range g.All {
		if strings.EqualFold(n, target) {
			return n, nil
		}
	}
	return "", fmt.Errorf("node %q not in group %q (see `uclash node ls %s`)", target, g.Name, g.Name)
}

func pickGroup(ps *mihomoapi.Proxies, target string) (string, error) {
	var candidates []string
	for name, p := range ps.Proxies {
		if !isSelector(p.Type) || name == "GLOBAL" {
			continue
		}
		for _, n := range p.All {
			if n == target || strings.EqualFold(n, target) {
				candidates = append(candidates, name)
				break
			}
		}
	}
	if len(candidates) == 0 {
		if _, err := strconv.Atoi(target); err == nil {
			return "", fmt.Errorf("index %q needs an explicit group: uclash node use <group> <%s>", target, target)
		}
		return "", fmt.Errorf("node %q not found in any select group (see `uclash node ls`)", target)
	}
	for _, c := range candidates {
		if c == "PROXY" {
			return c, nil
		}
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

func newNodeTestCmd() *cobra.Command {
	var (
		groupFlag string
		timeoutMS int
		testURL   string
	)
	cmd := &cobra.Command{
		Use:   "test [group]",
		Short: "Run a latency test for every node in a group",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			client, ok := a.Client()
			if !ok {
				return fmt.Errorf("core is not running (start it with `uclash start`)")
			}
			ps, err := client.GetProxies(cmd.Context())
			if err != nil {
				return err
			}
			group := groupFlag
			if group == "" && len(args) == 1 {
				group = args[0]
			}
			if group == "" {
				group = pickDefaultGroup(ps)
				if group == "" {
					return fmt.Errorf("no select group found; pass one explicitly (see `uclash node ls`)")
				}
			}
			g, ok := ps.Proxies[group]
			if !ok {
				return fmt.Errorf("group %q not found (see `uclash node ls`)", group)
			}
			var names []string
			for _, n := range g.All {
				switch strings.ToUpper(n) {
				case "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE":
					continue
				}
				names = append(names, n)
			}
			if len(names) == 0 {
				return fmt.Errorf("group %q has no testable nodes", group)
			}
			type result struct {
				name  string
				delay int
				err   error
			}
			results := make([]result, len(names))
			sem := make(chan struct{}, 16)
			var wg sync.WaitGroup
			for i, n := range names {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int, n string) {
					defer wg.Done()
					defer func() { <-sem }()
					d, derr := client.Delay(cmd.Context(), n, testURL, timeoutMS)
					results[i] = result{name: n, delay: d, err: derr}
				}(i, n)
			}
			wg.Wait()
			sort.Slice(results, func(i, j int) bool {
				a, b := results[i], results[j]
				if (a.err == nil) != (b.err == nil) {
					return a.err == nil
				}
				if a.err == nil && a.delay != b.delay {
					return a.delay < b.delay
				}
				return a.name < b.name
			})
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(w, "%s (%d nodes)\n", group, len(results))
			for _, r := range results {
				status := "timeout"
				if r.err == nil {
					status = fmt.Sprintf("%d ms", r.delay)
				}
				fmt.Fprintf(w, "  %s\t%s\n", r.name, status)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&groupFlag, "group", "", "group name (default: PROXY-like group with most nodes)")
	cmd.Flags().IntVar(&timeoutMS, "timeout", 3000, "per-node timeout in milliseconds")
	cmd.Flags().StringVar(&testURL, "url", "https://www.gstatic.com/generate_204", "test URL")
	return cmd
}

func pickDefaultGroup(ps *mihomoapi.Proxies) string {
	best, bestCount := "", 0
	for name, p := range ps.Proxies {
		if !isSelector(p.Type) || name == "GLOBAL" {
			continue
		}
		if name == "PROXY" {
			return name
		}
		if len(p.All) > bestCount {
			best, bestCount = name, len(p.All)
		}
	}
	return best
}
