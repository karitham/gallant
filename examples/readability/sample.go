package sample

func Process(items []int, threshold int) map[int][]int {
	out := map[int][]int{}
	for i, v := range items {
		if v > threshold {
			for j := i; j < len(items); j++ {
				if items[j]%2 == 0 && items[j] > v {
					out[i] = append(out[i], items[j])
					for k := j; k < len(items); k++ {
						if items[k] < 0 {
							out[i] = append(out[i], items[k])
						}
					}
				}
			}
		}
	}
	return out
}
