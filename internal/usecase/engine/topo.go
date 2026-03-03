package engine

import "errors"

// TopologicalSort mengurutkan node dari hulu ke hilir menggunakan Algoritma Kahn
func TopologicalSort(graph *VisualGraph) ([]Node, error) {
	inDegree := make(map[string]int)
	graphMap := make(map[string][]string)
	nodeMap := make(map[string]Node)

	// Inisialisasi peta node dan derajat masuk (in-degree) ke 0
	for _, n := range graph.Nodes {
		inDegree[n.ID] = 0
		nodeMap[n.ID] = n
	}

	// Bangun Adjacency List dan hitung derajat masuk tiap node
	for _, e := range graph.Edges {
		graphMap[e.Source] = append(graphMap[e.Source], e.Target)
		inDegree[e.Target]++
	}

	var queue []string
	// Cari node awal (derajat masuk = 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []Node
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:] // Dequeue
		sorted = append(sorted, nodeMap[curr])

		for _, neighbor := range graphMap[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor) // Enqueue jika tidak ada lagi dependensi
			}
		}
	}

	// Jika jumlah node yang terurut tidak sama dengan total node, ADA SIKLUS (Infinite Loop)
	if len(sorted) != len(graph.Nodes) {
		return nil, errors.New("FATAL: Siklus terdeteksi dalam grafik workflow. Eksekusi dibatalkan")
	}

	return sorted, nil
}
