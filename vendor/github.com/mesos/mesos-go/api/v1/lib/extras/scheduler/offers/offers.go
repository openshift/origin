package offers

import "github.com/mesos/mesos-go/api/v1/lib"

type (
	// Slice is a convenience type wrapper for a slice of mesos Offer messages
	Slice []mesos.Offer

	// Index is a convenience type wrapper for a dictionary of Offer messages
	Index map[interface{}]*mesos.Offer

	// KeyFunc generates a key used for indexing offers
	KeyFunc func(*mesos.Offer) interface{}
)

// IDs extracts the ID field from a Slice of offers
func (offers Slice) IDs() []mesos.OfferID {
	ids := make([]mesos.OfferID, len(offers))
	for i := range offers {
		ids[i] = offers[i].ID
	}
	return ids
}

// IDs extracts the ID field from a Index of offers
func (offers Index) IDs() []mesos.OfferID {
	ids := make([]mesos.OfferID, 0, len(offers))
	for _, offer := range offers {
		ids = append(ids, offer.GetID())
	}
	return ids
}

// Find returns the first Offer that passes the given filter function, or else nil if
// there are no passing offers.
func (offers Slice) Find(filter Filter) *mesos.Offer {
	for i := range offers {
		offer := &offers[i]
		if filter.Accept(offer) {
			return offer
		}
	}
	return nil
}

// Find returns the first Offer that passes the given filter function, or else nil if
// there are no passing offers.
func (offers Index) Find(filter Filter) *mesos.Offer {
	for _, offer := range offers {
		if filter.Accept(offer) {
			return offer
		}
	}
	return nil
}

// Filter returns the subset of the Slice that matches the given filter.
func (offers Slice) Filter(filter Filter) (result Slice) {
	if sz := len(result); sz > 0 {
		result = make(Slice, 0, sz)
		for i := range offers {
			if filter.Accept(&offers[i]) {
				result = append(result, offers[i])
			}
		}
	}
	return
}

// Filter returns the subset of the Index that matches the given filter.
func (offers Index) Filter(filter Filter) (result Index) {
	if sz := len(result); sz > 0 {
		result = make(Index, sz)
		for id, offer := range offers {
			if filter.Accept(offer) {
				result[id] = offer
			}
		}
	}
	return
}

// FilterNot returns the subset of the Slice that does not match the given filter.
func (offers Slice) FilterNot(filter Filter) Slice { return offers.Filter(not(filter)) }

// FilterNot returns the subset of the Index that does not match the given filter.
func (offers Index) FilterNot(filter Filter) Index { return offers.Filter(not(filter)) }

// DefaultKeyFunc indexes offers by their OfferID.
var DefaultKeyFunc = KeyFunc(KeyFuncByOfferID)

func KeyFuncByOfferID(o *mesos.Offer) interface{} { return o.GetID() }

// NewIndex returns a new Index constructed from the list of mesos offers.
// If the KeyFunc is nil then offers are indexed by DefaultKeyFunc.
// The values of the returned Index are pointers to (not copies of) the offers of the slice receiver.
func NewIndex(slice []mesos.Offer, kf KeyFunc) Index {
	if slice == nil {
		return nil
	}
	if kf == nil {
		kf = DefaultKeyFunc
	}
	index := make(Index, len(slice))
	for i := range slice {
		offer := &slice[i]
		index[kf(offer)] = offer
	}
	return index
}

// ToSlice returns a Slice from the offers in the Index.
// The returned slice will contain shallow copies of the offers from the Index.
func (offers Index) ToSlice() (slice Slice) {
	if sz := len(offers); sz > 0 {
		slice = make(Slice, 0, sz)
		for _, offer := range offers {
			slice = append(slice, *offer)
		}
	}
	return
}

/*

type Reducer func(_, _ *Offer) *Offer

func (slice Slice) Reduce(r Reducer) (result Offer) {
	if r == nil {
		return
	}
	acc := &result
	for i := range slice {
		acc = r(&result, &slice[i])
	}
	if acc == nil {
		result = Offer{}
	} else {
		result = *acc
	}
	return
}

func (index Index) Reduce(r Reducer) (result *Offer) {
	if r == nil {
		return
	}
	for i := range index {
		result = r(result, index[i])
	}
	return
}

func (slice Slice) GroupBy(kf KeyFunc) map[interface{}]Slice {
	if kf == nil {
		panic("keyFunc must not be nil")
	}
	if len(slice) == 0 {
		return nil
	}
	result := make(map[interface{}]Slice)
	for i := range slice {
		groupKey := kf(&slice[i])
		result[groupKey] = append(result[groupKey], slice[i])
	}
	return result
}

func (index Index) GroupBy(kf KeyFunc) map[interface{}]Index {
	if kf == nil {
		panic("keyFunc must not be nil")
	}
	if len(index) == 0 {
		return nil
	}
	result := make(map[interface{}]Index)
	for i, offer := range index {
		groupKey := kf(offer)
		group, ok := result[groupKey]
		if !ok {
			group = make(Index)
			result[groupKey] = group
		}
		group[i] = offer
	}
	return result
}

func (index Index) Partition(f Filter) (accepted, rejected Index) {
	if f == nil {
		return index, nil
	}
	if len(index) > 0 {
		accepted, rejected = make(Index), make(Index)
		for id, offer := range index {
			if f.Accept(offer) {
				accepted[id] = offer
			} else {
				rejected[id] = offer
			}
		}
	}
	return
}

func (s Slice) Partition(f Filter) (accepted, rejected []int) {
	if f == nil {
		accepted = make([]int, len(s))
		for i := range s {
			accepted[i] = i
		}
		return
	}
	if sz := len(s); sz > 0 {
		accepted, rejected = make([]int, 0, sz/2), make([]int, 0, sz/2)
		for i := range s {
			offer := &s[i]
			if f.Accept(offer) {
				accepted = append(accepted, i)
			} else {
				rejected = append(rejected, i)
			}
		}
	}
	return
}

func (index Index) Reindex(kf KeyFunc) Index {
	sz := len(index)
	if kf == nil || sz == 0 {
		return index
	}
	result := make(Index, sz)
	for _, offer := range index {
		key := kf(offer)
		result[key] = offer
	}
	return result
}
*/
