//go:build windows

package main

import (
	"fmt"
	"log"
	"net/netip"
	"os"
	"text/tabwriter"

	"github.com/bnkrr/winroute"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wroute",
	Short: "A CLI tool to manage Windows routes using the winroute package.",
	Long: `wroute is a command-line interface that provides easy access to
the functionalities of the winroute package, allowing you to get, add,
and delete routes on a Windows system.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func main() {
	Execute()
}

// ---- getCmd ----
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get and filter Windows routes",
	Long:  `Retrieves the system's routing table. You can apply filters to narrow down the results.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var filters []winroute.FilterOption

		// Destination Prefix Filter
		if destStr, _ := cmd.Flags().GetString("destination"); destStr != "" {
			prefix, err := netip.ParsePrefix(destStr)
			if err != nil {
				return fmt.Errorf("invalid destination prefix '%s': %w", destStr, err)
			}
			filters = append(filters, winroute.WithDestinationPrefix(prefix))
		}

		// Interface Index Filter
		if ifIndex, _ := cmd.Flags().GetUint32("if-index"); ifIndex > 0 {
			filters = append(filters, winroute.WithInterfaceIndex(ifIndex))
		}

		// Interface Alias Filter
		if ifAlias, _ := cmd.Flags().GetString("if-alias"); ifAlias != "" {
			filters = append(filters, winroute.WithInterfaceAlias(ifAlias))
		}

		// Metric Filter
		if cmd.Flags().Changed("metric") {
			metric, _ := cmd.Flags().GetUint32("metric")
			filters = append(filters, winroute.WithMetric(metric))
		}

		routes, err := winroute.GetRoutes(filters...)
		if err != nil {
			return fmt.Errorf("failed to get routes: %w", err)
		}

		if len(routes) == 0 {
			fmt.Println("No routes found matching the criteria.")
			return nil
		}

		// Print results in a table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "DESTINATION\tNEXT_HOP\tMETRIC\tIFACE_INDEX\tIFACE_ALIAS")
		for _, route := range routes {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n",
				route.Destination,
				route.NextHop,
				route.Metric,
				route.Interface.Index,
				route.Interface.Alias,
			)
		}
		return w.Flush()
	},
}

// ---- addCmd ----
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new route",
	Long:  `Adds a new, non-persistent route to the Windows routing table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		destStr, _ := cmd.Flags().GetString("destination")
		nextHopStr, _ := cmd.Flags().GetString("next-hop")
		ifIndex, _ := cmd.Flags().GetUint32("if-index")
		metric, _ := cmd.Flags().GetUint32("metric")

		destination, err := netip.ParsePrefix(destStr)
		if err != nil {
			return fmt.Errorf("invalid destination prefix '%s': %w", destStr, err)
		}

		nextHop, err := netip.ParseAddr(nextHopStr)
		if err != nil {
			return fmt.Errorf("invalid next-hop address '%s': %w", nextHopStr, err)
		}

		err = winroute.AddRoute(destination, nextHop, ifIndex, metric)
		if err != nil {
			return err
		}

		return nil // On success, print nothing.
	},
}

// ---- deleteRouteCmd ----
var deleteRouteCmd = &cobra.Command{
	Use:   "delete-one",
	Short: "Delete a single, specific route",
	Long:  `Deletes a single route by precisely matching its destination, next hop, and interface index.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		destStr, _ := cmd.Flags().GetString("destination")
		nextHopStr, _ := cmd.Flags().GetString("next-hop")
		ifIndex, _ := cmd.Flags().GetUint32("if-index")

		destination, err := netip.ParsePrefix(destStr)
		if err != nil {
			return fmt.Errorf("invalid destination prefix '%s': %w", destStr, err)
		}

		nextHop, err := netip.ParseAddr(nextHopStr)
		if err != nil {
			return fmt.Errorf("invalid next-hop address '%s': %w", nextHopStr, err)
		}

		// This calls the specific DeleteRoute function, not the filter-based one.
		err = winroute.DeleteRoute(destination, nextHop, ifIndex)
		if err != nil {
			return err
		}

		return nil // On success, print nothing.
	},
}

// ---- deleteCmd ----
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete routes based on filters",
	Long: `Deletes one or more routes from the routing table based on the provided filters.
At least one filter must be specified to prevent accidental deletion of all routes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var allOpts []any
		var filterCount int

		// Build filter and option slices based on flags
		if destStr, _ := cmd.Flags().GetString("destination"); destStr != "" {
			prefix, err := netip.ParsePrefix(destStr)
			if err != nil {
				return fmt.Errorf("invalid destination prefix '%s': %w", destStr, err)
			}
			filter := winroute.WithDestinationPrefix(prefix)
			allOpts = append(allOpts, filter)
			filterCount++
		}
		if ifIndex, _ := cmd.Flags().GetUint32("if-index"); ifIndex > 0 {
			filter := winroute.WithInterfaceIndex(ifIndex)
			allOpts = append(allOpts, filter)
			filterCount++
		}
		if ifAlias, _ := cmd.Flags().GetString("if-alias"); ifAlias != "" {
			filter := winroute.WithInterfaceAlias(ifAlias)
			allOpts = append(allOpts, filter)
			filterCount++
		}
		if cmd.Flags().Changed("metric") {
			metric, _ := cmd.Flags().GetUint32("metric")
			filter := winroute.WithMetric(metric)
			allOpts = append(allOpts, filter)
			filterCount++
		}

		if filterCount == 0 {
			return fmt.Errorf("at least one filter (--destination, --if-index, --if-alias, --metric) must be provided for deletion")
		}

		if stopOnError, _ := cmd.Flags().GetBool("stop-on-error"); stopOnError {
			allOpts = append(allOpts, winroute.ErrorActionStop)
		}

		// Now, perform the deletion
		_, err := winroute.DeleteRoutes(allOpts...)
		if err != nil {
			// This happens for fatal errors or if StopOnError is used
			log.Printf("A fatal error occurred during deletion: %v", err)
			return err
		}

		return nil // On success, print nothing.
	},
}

// ---- init ----
func init() {
	// Add subcommands to root
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(deleteRouteCmd)
	rootCmd.AddCommand(deleteCmd)

	// Flags for 'get' command
	getCmd.Flags().StringP("destination", "d", "", "Filter by destination prefix (e.g., 192.168.1.0/24)")
	getCmd.Flags().Uint32P("if-index", "i", 0, "Filter by interface index")
	getCmd.Flags().StringP("if-alias", "a", "", "Filter by interface alias (case-insensitive)")
	getCmd.Flags().Uint32P("metric", "m", 0, "Filter by route metric")

	// Flags for 'add' command
	addCmd.Flags().StringP("destination", "d", "", "Destination prefix for the new route (e.g., 10.0.0.0/8)")
	addCmd.Flags().StringP("next-hop", "n", "", "Next hop address for the new route (e.g., 192.168.1.1)")
	addCmd.Flags().Uint32P("if-index", "i", 0, "Interface index for the new route")
	addCmd.Flags().Uint32P("metric", "m", 0, "Metric for the new route (lower is more preferred)")
	addCmd.MarkFlagRequired("destination")
	addCmd.MarkFlagRequired("next-hop")
	addCmd.MarkFlagRequired("if-index")

	// Flags for 'delete-one' command
	deleteRouteCmd.Flags().StringP("destination", "d", "", "Destination prefix of the route to delete (e.g., 10.0.0.0/8)")
	deleteRouteCmd.Flags().StringP("next-hop", "n", "", "Next hop address of the route to delete (e.g., 192.168.1.1)")
	deleteRouteCmd.Flags().Uint32P("if-index", "i", 0, "Interface index of the route to delete")
	deleteRouteCmd.MarkFlagRequired("destination")
	deleteRouteCmd.MarkFlagRequired("next-hop")
	deleteRouteCmd.MarkFlagRequired("if-index")

	// Flags for 'delete' command
	deleteCmd.Flags().StringP("destination", "d", "", "Filter by destination prefix (e.g., 192.168.1.0/24)")
	deleteCmd.Flags().Uint32P("if-index", "i", 0, "Filter by interface index")
	deleteCmd.Flags().StringP("if-alias", "a", "", "Filter by interface alias (case-insensitive)")
	deleteCmd.Flags().Uint32P("metric", "m", 0, "Filter by route metric")
	deleteCmd.Flags().Bool("stop-on-error", false, "Stop the operation on the first error")
}
