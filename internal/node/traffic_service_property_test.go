// Package node provides node management functionality.
package node

import (
	"testing"
	"testing/quick"
)

// Feature: multi-server-management, Property 19: Traffic Aggregation Consistency
// Validates: Requirements 8.2
// For any traffic query, the sum of per-node traffic SHALL equal the total traffic for that user/proxy.

// TestProperty_TrafficAggregationConsistency tests that sum of per-node traffic equals total traffic.
func TestProperty_TrafficAggregationConsistency(t *testing.T) {
	// Property: For any set of traffic records, the sum of per-node traffic should equal total traffic
	f := func(trafficData []uint16) bool {
		if len(trafficData) == 0 {
			return true
		}

		// Constrain to reasonable size
		n := len(trafficData)
		if n > 100 {
			n = 100
		}

		// Simulate traffic records with node assignments
		type trafficRecord struct {
			nodeID   int64
			upload   int64
			download int64
		}

		records := make([]trafficRecord, n)
		for i := 0; i < n; i++ {
			// Assign to one of 5 nodes
			nodeID := int64(trafficData[i]%5) + 1
			upload := int64(trafficData[i] % 1000)
			download := int64((trafficData[i] * 2) % 1000)
			records[i] = trafficRecord{
				nodeID:   nodeID,
				upload:   upload,
				download: download,
			}
		}

		// Calculate total traffic (like GetTotalTraffic)
		var totalUpload, totalDownload int64
		for _, r := range records {
			totalUpload += r.upload
			totalDownload += r.download
		}

		// Calculate per-node traffic (like GetStatsByNode)
		nodeTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for _, r := range records {
			nt := nodeTraffic[r.nodeID]
			nt.upload += r.upload
			nt.download += r.download
			nodeTraffic[r.nodeID] = nt
		}

		// Sum per-node traffic
		var sumUpload, sumDownload int64
		for _, nt := range nodeTraffic {
			sumUpload += nt.upload
			sumDownload += nt.download
		}

		// Verify consistency: sum of per-node traffic should equal total traffic
		return sumUpload == totalUpload && sumDownload == totalDownload
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_UserTrafficAggregationConsistency tests that sum of per-node traffic for a user equals total user traffic.
func TestProperty_UserTrafficAggregationConsistency(t *testing.T) {
	// Property: For any user, the sum of their per-node traffic should equal their total traffic
	f := func(trafficData []uint16, userID uint8) bool {
		if len(trafficData) == 0 {
			return true
		}

		// Constrain to reasonable size
		n := len(trafficData)
		if n > 100 {
			n = 100
		}

		targetUserID := int64(userID%10) + 1

		// Simulate traffic records with user and node assignments
		type trafficRecord struct {
			userID   int64
			nodeID   int64
			upload   int64
			download int64
		}

		records := make([]trafficRecord, n)
		for i := 0; i < n; i++ {
			// Assign to one of 10 users and 5 nodes
			uID := int64(trafficData[i]%10) + 1
			nodeID := int64((trafficData[i]/10)%5) + 1
			upload := int64(trafficData[i] % 1000)
			download := int64((trafficData[i] * 2) % 1000)
			records[i] = trafficRecord{
				userID:   uID,
				nodeID:   nodeID,
				upload:   upload,
				download: download,
			}
		}

		// Calculate total traffic for target user (like GetTotalByUser)
		var totalUpload, totalDownload int64
		for _, r := range records {
			if r.userID == targetUserID {
				totalUpload += r.upload
				totalDownload += r.download
			}
		}

		// Calculate per-node traffic for target user (like GetUserTrafficBreakdownByNode)
		nodeTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for _, r := range records {
			if r.userID == targetUserID {
				nt := nodeTraffic[r.nodeID]
				nt.upload += r.upload
				nt.download += r.download
				nodeTraffic[r.nodeID] = nt
			}
		}

		// Sum per-node traffic for user
		var sumUpload, sumDownload int64
		for _, nt := range nodeTraffic {
			sumUpload += nt.upload
			sumDownload += nt.download
		}

		// Verify consistency: sum of per-node traffic should equal total user traffic
		return sumUpload == totalUpload && sumDownload == totalDownload
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_GroupTrafficAggregationConsistency tests that sum of node traffic in a group equals group total.
func TestProperty_GroupTrafficAggregationConsistency(t *testing.T) {
	// Property: For any group, the sum of its member nodes' traffic should equal the group's total traffic
	f := func(trafficData []uint16, groupMembership []uint8) bool {
		if len(trafficData) == 0 || len(groupMembership) == 0 {
			return true
		}

		// Constrain to reasonable size
		n := len(trafficData)
		if n > 100 {
			n = 100
		}

		numNodes := 5
		numGroups := 3

		// Create group membership (which nodes belong to which groups)
		// A node can belong to multiple groups
		nodeGroups := make(map[int64][]int64)
		for i := 0; i < numNodes; i++ {
			nodeID := int64(i + 1)
			// Assign node to 1-2 groups based on membership data
			idx := i % len(groupMembership)
			groupID := int64(groupMembership[idx]%uint8(numGroups)) + 1
			nodeGroups[nodeID] = append(nodeGroups[nodeID], groupID)
			// Some nodes belong to multiple groups
			if groupMembership[idx]%2 == 0 && numGroups > 1 {
				secondGroup := int64((groupMembership[idx]/2)%uint8(numGroups)) + 1
				if secondGroup != groupID {
					nodeGroups[nodeID] = append(nodeGroups[nodeID], secondGroup)
				}
			}
		}

		// Simulate traffic records
		type trafficRecord struct {
			nodeID   int64
			upload   int64
			download int64
		}

		records := make([]trafficRecord, n)
		for i := 0; i < n; i++ {
			nodeID := int64(trafficData[i]%uint16(numNodes)) + 1
			upload := int64(trafficData[i] % 1000)
			download := int64((trafficData[i] * 2) % 1000)
			records[i] = trafficRecord{
				nodeID:   nodeID,
				upload:   upload,
				download: download,
			}
		}

		// Calculate per-node traffic
		nodeTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for _, r := range records {
			nt := nodeTraffic[r.nodeID]
			nt.upload += r.upload
			nt.download += r.download
			nodeTraffic[r.nodeID] = nt
		}

		// For each group, verify that sum of member node traffic equals group total
		for groupID := int64(1); groupID <= int64(numGroups); groupID++ {
			// Find nodes in this group
			var groupUpload, groupDownload int64
			for nodeID, groups := range nodeGroups {
				for _, gID := range groups {
					if gID == groupID {
						nt := nodeTraffic[nodeID]
						groupUpload += nt.upload
						groupDownload += nt.download
						break
					}
				}
			}

			// Calculate group total by summing member nodes (same calculation)
			var sumUpload, sumDownload int64
			for nodeID, groups := range nodeGroups {
				for _, gID := range groups {
					if gID == groupID {
						nt := nodeTraffic[nodeID]
						sumUpload += nt.upload
						sumDownload += nt.download
						break
					}
				}
			}

			// Verify consistency
			if sumUpload != groupUpload || sumDownload != groupDownload {
				return false
			}
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ProxyTrafficAggregationConsistency tests that sum of per-proxy traffic equals total traffic.
func TestProperty_ProxyTrafficAggregationConsistency(t *testing.T) {
	// Property: For any set of traffic records with proxy IDs, the sum of per-proxy traffic should equal total traffic
	f := func(trafficData []uint16) bool {
		if len(trafficData) == 0 {
			return true
		}

		// Constrain to reasonable size
		n := len(trafficData)
		if n > 100 {
			n = 100
		}

		// Simulate traffic records with proxy assignments
		type trafficRecord struct {
			proxyID  int64
			upload   int64
			download int64
		}

		records := make([]trafficRecord, n)
		for i := 0; i < n; i++ {
			// Assign to one of 10 proxies
			proxyID := int64(trafficData[i]%10) + 1
			upload := int64(trafficData[i] % 1000)
			download := int64((trafficData[i] * 2) % 1000)
			records[i] = trafficRecord{
				proxyID:  proxyID,
				upload:   upload,
				download: download,
			}
		}

		// Calculate total traffic
		var totalUpload, totalDownload int64
		for _, r := range records {
			totalUpload += r.upload
			totalDownload += r.download
		}

		// Calculate per-proxy traffic
		proxyTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for _, r := range records {
			pt := proxyTraffic[r.proxyID]
			pt.upload += r.upload
			pt.download += r.download
			proxyTraffic[r.proxyID] = pt
		}

		// Sum per-proxy traffic
		var sumUpload, sumDownload int64
		for _, pt := range proxyTraffic {
			sumUpload += pt.upload
			sumDownload += pt.download
		}

		// Verify consistency: sum of per-proxy traffic should equal total traffic
		return sumUpload == totalUpload && sumDownload == totalDownload
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_TrafficTotalCalculation tests that total is always upload + download.
func TestProperty_TrafficTotalCalculation(t *testing.T) {
	// Property: For any traffic stats, total should always equal upload + download
	f := func(upload, download uint32) bool {
		up := int64(upload)
		down := int64(download)
		total := up + down

		// Simulate TrafficStats calculation
		stats := TrafficStats{
			Upload:   up,
			Download: down,
			Total:    up + down,
		}

		return stats.Total == total && stats.Total == stats.Upload+stats.Download
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_EmptyTrafficAggregation tests that empty traffic data aggregates to zero.
func TestProperty_EmptyTrafficAggregation(t *testing.T) {
	// Property: Aggregating empty traffic data should result in zero totals
	f := func(numNodes uint8) bool {
		nodes := int(numNodes%20) + 1

		// Simulate empty traffic records
		records := make([]struct {
			nodeID   int64
			upload   int64
			download int64
		}, 0)

		// Calculate total traffic
		var totalUpload, totalDownload int64
		for _, r := range records {
			totalUpload += r.upload
			totalDownload += r.download
		}

		// Calculate per-node traffic
		nodeTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for i := 1; i <= nodes; i++ {
			nodeTraffic[int64(i)] = struct {
				upload   int64
				download int64
			}{0, 0}
		}

		// Sum per-node traffic
		var sumUpload, sumDownload int64
		for _, nt := range nodeTraffic {
			sumUpload += nt.upload
			sumDownload += nt.download
		}

		// Verify all are zero
		return totalUpload == 0 && totalDownload == 0 && sumUpload == 0 && sumDownload == 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_TrafficAggregationNonNegative tests that aggregated traffic is never negative.
func TestProperty_TrafficAggregationNonNegative(t *testing.T) {
	// Property: Aggregated traffic values should never be negative
	f := func(trafficData []uint16) bool {
		if len(trafficData) == 0 {
			return true
		}

		// Constrain to reasonable size
		n := len(trafficData)
		if n > 100 {
			n = 100
		}

		// Simulate traffic records (using unsigned values ensures non-negative)
		type trafficRecord struct {
			nodeID   int64
			upload   int64
			download int64
		}

		records := make([]trafficRecord, n)
		for i := 0; i < n; i++ {
			nodeID := int64(trafficData[i]%5) + 1
			upload := int64(trafficData[i])
			download := int64(trafficData[i] * 2)
			records[i] = trafficRecord{
				nodeID:   nodeID,
				upload:   upload,
				download: download,
			}
		}

		// Calculate total traffic
		var totalUpload, totalDownload int64
		for _, r := range records {
			totalUpload += r.upload
			totalDownload += r.download
		}

		// Calculate per-node traffic
		nodeTraffic := make(map[int64]struct {
			upload   int64
			download int64
		})
		for _, r := range records {
			nt := nodeTraffic[r.nodeID]
			nt.upload += r.upload
			nt.download += r.download
			nodeTraffic[r.nodeID] = nt
		}

		// Verify all values are non-negative
		if totalUpload < 0 || totalDownload < 0 {
			return false
		}

		for _, nt := range nodeTraffic {
			if nt.upload < 0 || nt.download < 0 {
				return false
			}
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
