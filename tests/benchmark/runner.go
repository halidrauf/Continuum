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

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// GlobalStats matches the structure from server.go
type GlobalStats struct {
	TotalTasks      int     `json:"total_tasks"`
	PendingTasks    int     `json:"pending_tasks"`
	RunningTasks    int     `json:"running_tasks"`
	CompletedTasks  int     `json:"completed_tasks"`
	FailedTasks     int     `json:"failed_tasks"`
	AvgExecutionSec float64 `json:"avg_execution_seconds"`
	ThroughputTasks float64 `json:"throughput_tasks_per_hour"`
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func main() {
	suite := flag.String("suite", "", "Benchmark suite to run (cpu, network)")
	dbHost := flag.String("db_host", "localhost", "Database host")
	apiHost := flag.String("api_host", "localhost", "Worker API host")
	apiPort := flag.String("api_port", "8080", "Worker API port")
	flag.Parse()

	if *suite == "" {
		fmt.Printf("%sPlease specify a suite using --suite=[cpu|network|mixed|realistic|security|all]%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	// Load DB config from .env or defaults
	_ = godotenv.Load("../../.env")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" { dbUser = "user" }
	if dbPass == "" { dbPass = "password" }
	if dbName == "" { dbName = "continuum" }

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=require",
		dbUser, dbPass, dbName, *dbHost)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("%sFailed to connect to DB: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer db.Close()

	// 2. Load Scenario
	scenarioFile := fmt.Sprintf("scenarios/%s_stress.sql", *suite)
	switch *suite {
	case "network":
		scenarioFile = "scenarios/network_io.sql"
	case "cpu":
		scenarioFile = "scenarios/cpu_stress.sql"
	case "mixed":
		scenarioFile = "scenarios/mixed_load.sql"
	case "realistic":
		scenarioFile = "scenarios/realistic.sql"
	case "all":
		scenarioFile = "scenarios/all.sql"
	case "security":
		scenarioFile = "scenarios/security_probe.sql"
	}

	content, err := os.ReadFile(scenarioFile)
	if err != nil {
		fmt.Printf("%sError reading scenario file %s: %v%s\n", colorRed, scenarioFile, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s%s %s CONTINUUM BENCHMARK %s %s%s\n", colorCyan, colorBold, ">>", "SUITE: "+*suite, "<<", colorReset)
	
	// Get Baseline Stats
	initialStats, err := getGlobalStats(*apiHost, *apiPort)
	if err != nil {
		fmt.Printf("%s[WARN]%s Could not get initial stats: %v. Metrics might be absolute.\n", colorYellow, colorReset, err)
	}

	// 3. Execute SQL to Insert Tasks
	_, err = db.Exec(string(content))
	if err != nil {
		fmt.Printf("%s[ERR]%s Failed to insert tasks: %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}
	fmt.Printf("%s[OK]%s Scenario loaded and tasks injected.\n\n", colorGreen, colorReset)

	// 4. Monitor Progress
	startTime := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond) // Faster updates for smoother UI
	defer ticker.Stop()

	fmt.Printf("%s%-10s %-12s %-10s %-10s %-10s%s\n", colorGray+colorBold, "ELAPSED", "COMPLETED", "FAILED", "RUNNING", "PENDING", colorReset)
	fmt.Println(colorGray + "------------------------------------------------------------" + colorReset)
	
	lastCompleted := 0

	for range ticker.C {
		stats, err := getGlobalStats(*apiHost, *apiPort)
		
		elapsed := time.Since(startTime).Round(time.Second).String()
		
		if err != nil {
			fmt.Printf("\r%-10s %s%-42s%s", 
				elapsed,
				colorRed, "Error: Connection Refused (Retrying...)", colorReset,
			)
			continue
		}

		deltaCompleted := stats.CompletedTasks - initialStats.CompletedTasks
		deltaFailed := stats.FailedTasks - initialStats.FailedTasks
		
		statusColor := colorGreen
		if deltaFailed > 0 {
			statusColor = colorRed
		}

		fmt.Printf("\r%-10s %s%-12d%s %s%-10d%s %s%-10d%s %-10d", 
			elapsed,
			colorGreen, deltaCompleted, colorReset,
			statusColor, deltaFailed, colorReset,
			colorYellow, stats.RunningTasks, colorReset,
			stats.PendingTasks,
		)

		if stats.RunningTasks == 0 && stats.PendingTasks == 0 && deltaCompleted+deltaFailed > 0 {
			if deltaCompleted+deltaFailed >= lastCompleted {
				fmt.Printf("\n%s------------------------------------------------------------%s\n", colorGray, colorReset)
				fmt.Printf("\n%s%s Benchmark Completed Successfully! %s%s\n", colorGreen, colorBold, "✓", colorReset)
				printReport(stats, initialStats, time.Since(startTime))
				break
			}
		}
		lastCompleted = deltaCompleted + deltaFailed
	}
}

func getGlobalStats(host, port string) (GlobalStats, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%s/global-status", host, port))
	if err != nil {
		return GlobalStats{}, err
	}
	defer resp.Body.Close()
	
	var stats GlobalStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return GlobalStats{}, err
	}
	return stats, nil
}

func printReport(final, initial GlobalStats, duration time.Duration) {
	totalProcessed := (final.CompletedTasks - initial.CompletedTasks) + (final.FailedTasks - initial.FailedTasks)
	tps := float64(totalProcessed) / duration.Seconds()
	
	successRate := 100.0
	if totalProcessed > 0 {
		successRate = (float64(final.CompletedTasks-initial.CompletedTasks) / float64(totalProcessed)) * 100
	}

	fmt.Println("\n" + colorCyan + colorBold + "┏━━━━━━━━━━━━━━━━━━━━━━ REPORT ━━━━━━━━━━━━━━━━━━━━━━┓" + colorReset)
	
	lineFmt := colorCyan + "┃" + colorReset + "  %-22s " + colorBold + "%-25s" + colorCyan + "┃" + colorReset
	
	fmt.Printf(lineFmt+"\n", "Duration:", duration.Truncate(time.Millisecond).String())
	fmt.Printf(lineFmt+"\n", "Total Tasks:", fmt.Sprintf("%d", totalProcessed))
	
	completedStr := fmt.Sprintf("%d", final.CompletedTasks-initial.CompletedTasks)
	fmt.Printf(colorCyan+"┃"+"  %-22s "+colorGreen+colorBold+"%-25s"+colorCyan+"┃"+colorReset+"\n", "  - Completed:", completedStr)
	
	failedVal := final.FailedTasks - initial.FailedTasks
	failedColor := colorGreen
	if failedVal > 0 {
		failedColor = colorRed
	}
	fmt.Printf(colorCyan+"┃"+"  %-22s "+failedColor+colorBold+"%-25s"+colorCyan+"┃"+colorReset+"\n", "  - Failed:", fmt.Sprintf("%d", failedVal))
	
	fmt.Printf(lineFmt+"\n", "Success Rate:", fmt.Sprintf("%.2f%%", successRate))
	fmt.Printf(lineFmt+"\n", "Throughput (TPS):", fmt.Sprintf("%.2f tasks/sec", tps))
	fmt.Printf(lineFmt+"\n", "Avg Latency:", fmt.Sprintf("%.2f ms", final.AvgExecutionSec*1000))
	fmt.Printf(lineFmt+"\n", "Hourly Capacity:", fmt.Sprintf("%.1f tasks/hr", final.ThroughputTasks))
	
	fmt.Println(colorCyan + colorBold + "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛" + colorReset)
}
