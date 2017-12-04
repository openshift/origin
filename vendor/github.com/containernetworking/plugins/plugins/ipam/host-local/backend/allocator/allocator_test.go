// Copyright 2017 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package allocator

import (
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	fakestore "github.com/containernetworking/plugins/plugins/ipam/host-local/backend/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type AllocatorTestCase struct {
	subnets      []string
	ipmap        map[string]string
	expectResult string
	lastIP       string
}

func mkalloc() IPAllocator {
	p := RangeSet{
		Range{Subnet: mustSubnet("192.168.1.0/29")},
	}
	p.Canonicalize()
	store := fakestore.NewFakeStore(map[string]string{}, map[string]net.IP{})

	alloc := IPAllocator{
		rangeset: &p,
		store:    store,
		rangeID:  "rangeid",
	}

	return alloc
}

func (t AllocatorTestCase) run(idx int) (*current.IPConfig, error) {
	fmt.Fprintln(GinkgoWriter, "Index:", idx)
	p := RangeSet{}
	for _, s := range t.subnets {
		subnet, err := types.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		p = append(p, Range{Subnet: types.IPNet(*subnet)})
	}

	Expect(p.Canonicalize()).To(BeNil())

	store := fakestore.NewFakeStore(t.ipmap, map[string]net.IP{"rangeid": net.ParseIP(t.lastIP)})

	alloc := IPAllocator{
		rangeset: &p,
		store:    store,
		rangeID:  "rangeid",
	}

	return alloc.Get("ID", nil)
}

var _ = Describe("host-local ip allocator", func() {
	Context("RangeIter", func() {
		It("should loop correctly from the beginning", func() {
			a := mkalloc()
			r, _ := a.GetIter()
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 2}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 3}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 4}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 5}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 6}))
			Expect(r.nextip()).To(BeNil())
		})

		It("should loop correctly from the end", func() {
			a := mkalloc()
			a.store.Reserve("ID", net.IP{192, 168, 1, 6}, a.rangeID)
			a.store.ReleaseByID("ID")
			r, _ := a.GetIter()
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 2}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 3}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 4}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 5}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 6}))
			Expect(r.nextip()).To(BeNil())
		})
		It("should loop correctly from the middle", func() {
			a := mkalloc()
			a.store.Reserve("ID", net.IP{192, 168, 1, 3}, a.rangeID)
			a.store.ReleaseByID("ID")
			r, _ := a.GetIter()
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 4}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 5}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 6}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 2}))
			Expect(r.nextip()).To(Equal(net.IP{192, 168, 1, 3}))
			Expect(r.nextip()).To(BeNil())
		})
	})

	Context("when has free ip", func() {
		It("should allocate ips in round robin", func() {
			testCases := []AllocatorTestCase{
				// fresh start
				{
					subnets:      []string{"10.0.0.0/29"},
					ipmap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "",
				},
				{
					subnets:      []string{"2001:db8:1::0/64"},
					ipmap:        map[string]string{},
					expectResult: "2001:db8:1::2",
					lastIP:       "",
				},
				{
					subnets:      []string{"10.0.0.0/30"},
					ipmap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "",
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "",
				},
				// next ip of last reserved ip
				{
					subnets:      []string{"10.0.0.0/29"},
					ipmap:        map[string]string{},
					expectResult: "10.0.0.6",
					lastIP:       "10.0.0.5",
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.4": "id",
						"10.0.0.5": "id",
					},
					expectResult: "10.0.0.6",
					lastIP:       "10.0.0.3",
				},
				// round robin to the beginning
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.6": "id",
					},
					expectResult: "10.0.0.2",
					lastIP:       "10.0.0.5",
				},
				// lastIP is out of range
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.128",
				},
				// subnet is completely full except for lastip
				// wrap around and reserve lastIP
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
						"10.0.0.4": "id",
						"10.0.0.5": "id",
						"10.0.0.6": "id",
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.3",
				},
				// alocate from multiple subnets
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30"},
					expectResult: "10.0.0.2",
					ipmap:        map[string]string{},
				},
				// advance to next subnet
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30"},
					lastIP:       "10.0.0.2",
					expectResult: "10.0.1.2",
					ipmap:        map[string]string{},
				},
				// Roll to start subnet
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30", "10.0.2.0/30"},
					lastIP:       "10.0.2.2",
					expectResult: "10.0.0.2",
					ipmap:        map[string]string{},
				},
			}

			for idx, tc := range testCases {
				res, err := tc.run(idx)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Address.IP.String()).To(Equal(tc.expectResult))
			}
		})

		It("should not allocate the broadcast address", func() {
			alloc := mkalloc()
			for i := 2; i < 7; i++ {
				res, err := alloc.Get("ID", nil)
				Expect(err).ToNot(HaveOccurred())
				s := fmt.Sprintf("192.168.1.%d/29", i)
				Expect(s).To(Equal(res.Address.String()))
				fmt.Fprintln(GinkgoWriter, "got ip", res.Address.String())
			}

			x, err := alloc.Get("ID", nil)
			fmt.Fprintln(GinkgoWriter, "got ip", x)
			Expect(err).To(HaveOccurred())
		})

		It("should allocate in a round-robin fashion", func() {
			alloc := mkalloc()
			res, err := alloc.Get("ID", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Address.String()).To(Equal("192.168.1.2/29"))

			err = alloc.Release("ID")
			Expect(err).ToNot(HaveOccurred())

			res, err = alloc.Get("ID", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Address.String()).To(Equal("192.168.1.3/29"))

		})

		Context("when requesting a specific IP", func() {
			It("must allocate the requested IP", func() {
				alloc := mkalloc()
				requestedIP := net.IP{192, 168, 1, 5}
				res, err := alloc.Get("ID", requestedIP)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Address.IP.String()).To(Equal(requestedIP.String()))
			})

			It("must fail when the requested IP is allocated", func() {
				alloc := mkalloc()
				requestedIP := net.IP{192, 168, 1, 5}
				res, err := alloc.Get("ID", requestedIP)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Address.IP.String()).To(Equal(requestedIP.String()))

				_, err = alloc.Get("ID", requestedIP)
				Expect(err).To(MatchError(`requested IP address 192.168.1.5 is not available in range set 192.168.1.1-192.168.1.6`))
			})

			It("must return an error when the requested IP is after RangeEnd", func() {
				alloc := mkalloc()
				(*alloc.rangeset)[0].RangeEnd = net.IP{192, 168, 1, 4}
				requestedIP := net.IP{192, 168, 1, 5}
				_, err := alloc.Get("ID", requestedIP)
				Expect(err).To(HaveOccurred())
			})

			It("must return an error when the requested IP is before RangeStart", func() {
				alloc := mkalloc()
				(*alloc.rangeset)[0].RangeStart = net.IP{192, 168, 1, 3}
				requestedIP := net.IP{192, 168, 1, 2}
				_, err := alloc.Get("ID", requestedIP)
				Expect(err).To(HaveOccurred())
			})
		})

	})
	Context("when out of ips", func() {
		It("returns a meaningful error", func() {
			testCases := []AllocatorTestCase{
				{
					subnets: []string{"10.0.0.0/30"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
					},
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
						"10.0.0.3": "id",
						"10.0.0.4": "id",
						"10.0.0.5": "id",
						"10.0.0.6": "id",
					},
				},
				{
					subnets: []string{"10.0.0.0/30", "10.0.1.0/30"},
					ipmap: map[string]string{
						"10.0.0.2": "id",
						"10.0.1.2": "id",
					},
				},
			}
			for idx, tc := range testCases {
				_, err := tc.run(idx)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(HavePrefix("no IP addresses available in range set"))
			}
		})
	})
})

// nextip is a convenience function used for testing
func (i *RangeIter) nextip() net.IP {
	c, _ := i.Next()
	if c == nil {
		return nil
	}

	return c.IP
}
