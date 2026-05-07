package room

var ValidCards = map[string]bool{
	"0": true, "0.5": true, "1": true, "2": true, "3": true, "5": true,
	"8": true, "13": true, "21": true, "?": true, "coffee": true,
}

// NumericValue returns the numeric value of a card if it is a number,
// along with ok=true. Non-numeric cards ("?" and "coffee") return ok=false.
func NumericValue(card string) (float64, bool) {
	switch card {
	case "0":
		return 0, true
	case "0.5":
		return 0.5, true
	case "1":
		return 1, true
	case "2":
		return 2, true
	case "3":
		return 3, true
	case "5":
		return 5, true
	case "8":
		return 8, true
	case "13":
		return 13, true
	case "21":
		return 21, true
	}
	return 0, false
}
