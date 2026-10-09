package domain

// CanTransition reports whether a status change from -> to is allowed by the
// order lifecycle. The only legal flow is the canonical order
// angefragt -> bestätigt -> in Arbeit -> fertig -> abgeholt: each transition
// must advance exactly one step, so a jump (e.g. angefragt -> abgeholt) and the
// repetition of a status are both rejected.
func CanTransition(from, to OrderStatus) bool {
	for i, status := range OrderStatusOrder {
		if status != from {
			continue
		}
		return i+1 < len(OrderStatusOrder) && OrderStatusOrder[i+1] == to
	}
	return false
}

// IsKnownStatus reports whether s is one of the lifecycle statuses. It is used
// to reject an unknown status value in a request body with 400.
func IsKnownStatus(s OrderStatus) bool {
	for _, status := range OrderStatusOrder {
		if status == s {
			return true
		}
	}
	return false
}
