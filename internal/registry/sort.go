package registry

import (
	"time"

	"slurmtui/internal/model"
)

func lessForColumn(id string) func(model.Job, model.Job) bool {
	switch id {
	case "job_id":
		return lessJobID
	case "elapsed_limit":
		return func(a, b model.Job) bool {
			aSet := firstState(a.States) != string(model.StatePending)
			bSet := firstState(b.States) != string(model.StatePending)
			if aSet != bSet {
				return aSet
			}
			if aSet && a.ElapsedSeconds != b.ElapsedSeconds {
				return a.ElapsedSeconds < b.ElapsedSeconds
			}
			return lessNoVal(a.TimeLimit, b.TimeLimit)
		}
	case "nodes":
		return func(a, b model.Job) bool { return lessNoVal(a.NodeCount, b.NodeCount) }
	case "end":
		return func(a, b model.Job) bool { return lessNoVal(a.EndTime, b.EndTime) }
	case "name":
		return func(a, b model.Job) bool { return lessText(a.Name, b.Name) }
	case "user":
		return func(a, b model.Job) bool { return lessText(a.UserName, b.UserName) }
	case "account":
		return func(a, b model.Job) bool { return lessText(a.Account, b.Account) }
	case "partition":
		return func(a, b model.Job) bool { return lessText(a.Partition, b.Partition) }
	case "qos":
		return func(a, b model.Job) bool { return lessText(a.QOS, b.QOS) }
	case "state":
		return func(a, b model.Job) bool { return lessText(firstState(a.States), firstState(b.States)) }
	case "node_reason":
		return func(a, b model.Job) bool {
			return lessText(nodeListReason(a, time.Time{}, nil), nodeListReason(b, time.Time{}, nil))
		}
	default:
		return func(a, b model.Job) bool { return a.DisplayID() < b.DisplayID() }
	}
}

func lessText(a, b string) bool {
	aMissing := a == "" || a == "—"
	bMissing := b == "" || b == "—"
	if aMissing != bMissing {
		return !aMissing
	}
	if aMissing {
		return false
	}
	return a < b
}

func lessNoVal(a, b model.NoValNumber) bool {
	if a.Set != b.Set {
		return a.Set
	}
	if !a.Set {
		return false
	}
	if a.Infinite != b.Infinite {
		return !a.Infinite
	}
	if a.Infinite {
		return false
	}
	return a.Number < b.Number
}

type numericJobID struct {
	base      int64
	component int64
	set       bool
}

func jobIDParts(job model.Job) numericJobID {
	if job.ArrayJobID.Set {
		return numericJobID{base: job.ArrayJobID.Number, component: job.ArrayTaskID.Number, set: job.ArrayTaskID.Set}
	}
	if job.HetJobID.Set {
		return numericJobID{base: job.HetJobID.Number, component: job.HetJobOffset.Number, set: job.HetJobOffset.Set}
	}
	return numericJobID{base: job.ID}
}

func lessJobID(a, b model.Job) bool {
	aID, bID := jobIDParts(a), jobIDParts(b)
	if aID.base != bID.base {
		return aID.base < bID.base
	}
	if aID.set != bID.set {
		return !aID.set
	}
	if aID.set && aID.component != bID.component {
		return aID.component < bID.component
	}
	return a.ID < b.ID
}

func JobIDLess(a, b model.Job) bool {
	return lessJobID(a, b)
}
