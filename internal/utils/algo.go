package utils

type Comparable interface {
	LessThan(Comparable) bool
	GreaterThan(Comparable) bool
}

func rbsearch(items []Comparable, target Comparable) (int, int, int) {
	i, j := 0, len(items)-1
	for i <= j {
		m := i + (j-i)/2
		if items[m].LessThan(target) {
			j = m - 1
		} else if items[m].GreaterThan(target) {
			i = m + 1
		} else {
			return m, i, j
		}
	}
	return len(items), i, j
}

func RFirstGreaterEqual(items []Comparable, target Comparable) int {
	r, _, j := rbsearch(items, target)
	if r == len(items) {
		return j
	}
	return r
}

func RFirstLessEqual(items []Comparable, target Comparable) int {
	r, i, _ := rbsearch(items, target)
	if r == len(items) {
		return i
	}
	return r
}
