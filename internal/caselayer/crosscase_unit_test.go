package caselayer

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("conversations across cases (no database)", func() {
	Describe("the list of cases", func() {
		It("trims each name, keeps each case once and puts them in byte order", func() {
			got, err := normalizeCases([]string{" orion ", "meridian", "orion", "Control"})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"Control", "meridian", "orion"}), "upper case sorts before lower case, as the database sorts the column")
		})

		It("keeps names in other scripts and sorts them by their bytes", func() {
			got, err := normalizeCases([]string{"قضیہ-۱", "case-2"})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"case-2", "قضیہ-۱"}))
		})

		It("needs two different cases and at most ten", func() {
			_, err := normalizeCases([]string{"a"})
			Expect(err).To(MatchError(ContainSubstring("at least 2 different cases")))
			_, err = normalizeCases([]string{"a", " a "})
			Expect(err).To(MatchError(ContainSubstring("at least 2 different cases")), "the same case twice is one case")
			_, err = normalizeCases(nil)
			Expect(err).To(HaveOccurred())
			ten := []string{"c01", "c02", "c03", "c04", "c05", "c06", "c07", "c08", "c09", "c10"}
			got, err := normalizeCases(ten)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(10))
			_, err = normalizeCases(append(ten, "c11"))
			Expect(err).To(MatchError(ContainSubstring("at most 10 cases")))
		})

		It("refuses an empty name, a control character and a name longer than a case may have", func() {
			for _, bad := range [][]string{{"a", ""}, {"a", "   "}, {"a", "b\x00"}, {"a", "b\nc"}, {"a", strings.Repeat("x", 129)}} {
				_, err := normalizeCases(bad)
				Expect(err).To(HaveOccurred(), "%q", bad)
				var e *Error
				Expect(err).To(BeAssignableToTypeOf(e))
				Expect(err.(*Error).Status).To(Equal(400))
			}
			_, err := normalizeCases([]string{"a", strings.Repeat("x", 128)})
			Expect(err).NotTo(HaveOccurred(), "128 characters is allowed")
		})

		It("keeps the whole list inside what the table accepts (1,280 bytes), counting bytes, not characters", func() {
			long := strings.Repeat("ق", 127) // with the letter in front: 128 characters, 255 bytes
			var names []string
			for i := 0; i < 6; i++ {
				names = append(names, string(rune('a'+i))+long)
			}
			_, err := normalizeCases(names)
			Expect(err).To(MatchError(ContainSubstring("1,280 bytes")))
		})
	})

	Describe("what a person is in a conversation across cases", func() {
		It("is the weakest of their roles, and nothing when they have none in one of the cases", func() {
			Expect(weakestRole(map[string]Role{"a": RoleOwner, "b": RoleAnalyst})).To(Equal(RoleAnalyst))
			Expect(weakestRole(map[string]Role{"a": RoleOpen, "b": RoleLead})).To(Equal(RoleOpen))
			Expect(weakestRole(map[string]Role{"a": RoleViewer, "b": RoleOwner})).To(Equal(RoleViewer))
			Expect(weakestRole(map[string]Role{"a": RoleAuditor, "b": RoleOwner})).To(Equal(RoleAuditor))
			Expect(weakestRole(map[string]Role{"a": RoleOwner, "b": RoleNone})).To(Equal(RoleNone))
			Expect(weakestRole(map[string]Role{"a": Role("something new")})).To(Equal(RoleNone), "a role the layer does not know counts as none")
			Expect(weakestRole(map[string]Role{})).To(Equal(RoleNone))
			Expect(weakestRole(nil)).To(Equal(RoleNone))
			Expect(weakestRole(map[string]Role{"a": RoleAnalyst}).CanContribute()).To(BeTrue())
			Expect(weakestRole(map[string]Role{"a": RoleAnalyst, "b": RoleReviewer}).CanContribute()).To(BeFalse(), "a reviewer in one case cannot ask in it, so the conversation cannot be asked")
		})

		It("is an auditor only when they are an auditor of every case", func() {
			Expect(allRoles(map[string]Role{"a": RoleAuditor, "b": RoleAuditor}, RoleAuditor)).To(BeTrue())
			Expect(allRoles(map[string]Role{"a": RoleAuditor, "b": RoleOwner}, RoleAuditor)).To(BeFalse(), "an auditor of one case and the owner of another reads no private threads across them")
			Expect(allRoles(map[string]Role{"a": RoleAuditor, "b": RoleOpen}, RoleAuditor)).To(BeFalse())
			Expect(allRoles(map[string]Role{"a": RoleOwner, "b": RoleLead}, RoleOwner, RoleLead)).To(BeTrue())
			Expect(allRoles(map[string]Role{}, RoleAuditor)).To(BeFalse(), "no case at all is not all cases")
		})
	})

	Describe("the sentence that states the scope of an answer", func() {
		It("names the cases asked and counts the answers", func() {
			g := Group{Cases: []GroupCase{{CaseID: "meridian"}, {CaseID: "orion"}, {CaseID: "control"}}, Answered: 2, NotAnswered: 1}
			Expect(groupSummary(g)).To(Equal("Asked in 3 cases (meridian, orion, control): answered in 2, not answered in 1."))
			all := Group{Cases: []GroupCase{{CaseID: "a"}, {CaseID: "b"}}, Answered: 2}
			Expect(groupSummary(all)).To(Equal("Asked in 2 cases (a, b): answered in 2, not answered in 0."))
		})

		It("counts a case whose turn failed or was never written as not answered", func() {
			done, other, failed, none := "completed", "answered", "failed", "abstained"
			completed := &Turn{State: done, Outcome: &other}
			broke := &Turn{State: failed, Outcome: &failed}
			abstained := &Turn{State: done, Outcome: &none}
			g := Group{Cases: []GroupCase{{CaseID: "a", Turn: completed}, {CaseID: "b", Turn: broke}, {CaseID: "c", Turn: abstained}, {CaseID: "d", Error: "no model"}}}
			countGroup(&g)
			Expect(g.Answered).To(Equal(1))
			Expect(g.NotAnswered).To(Equal(3))
			Expect(g.Summary).To(Equal("Asked in 4 cases (a, b, c, d): answered in 1, not answered in 3."))
		})
	})

	Describe("the events of a question put to several cases", func() {
		It("marks each event with its case and group, keeps the clock of the whole question and leaves the single 'done' to the group", func() {
			var seen []Event
			inner := SinkFunc(func(e Event) { seen = append(seen, e) })
			cs := caseSink{inner: inner, caseID: "orion", groupID: "g-1", offsetMS: 1500}
			cs.Send(Event{Event: "phase", Phase: "engine", AtMS: 40})
			cs.Send(Event{Event: "heartbeat", AtMS: 2040})
			cs.Send(Event{Event: "turn", AtMS: 2100})
			cs.Send(Event{Event: "done", AtMS: 2100})
			Expect(seen).To(HaveLen(3), "the end of one case is not the end of the question")
			for _, e := range seen {
				Expect(e.CaseID).To(Equal("orion"))
				Expect(e.GroupID).To(Equal("g-1"))
			}
			Expect(seen[0].AtMS).To(Equal(int64(1540)))
			Expect(seen[2].AtMS).To(Equal(int64(3600)))
			Expect(seen[2].Event).To(Equal("turn"))
		})
	})
})
