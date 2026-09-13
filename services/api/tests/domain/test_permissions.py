"""The single permission table, spec section 9."""

from __future__ import annotations

import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[2]))

from app.domain import permissions  # noqa: E402
from app.domain.permissions import (  # noqa: E402
    ACTIONS,
    AuthorizationFacts,
    PermissionError_,
    can,
    denial_reason,
)


def facts(roles, proven=(), actor="u1", resource="r1"):
    """Build facts the way an adapter would, with provenance recorded."""
    return AuthorizationFacts(
        actor_id=actor,
        roles=frozenset(roles),
        resource_id=resource,
        proven=frozenset(proven),
        provenance="test_fixture",
    )


class TableShape(unittest.TestCase):
    def test_unknown_action_raises_rather_than_silently_denying(self):
        """A typo in an action name must fail loudly.

        Returning False would turn a misspelled permission check into a silent
        deny that looks exactly like a correct deny in production.
        """
        with self.assertRaises(PermissionError_) as caught:
            can("publsh_batch", facts({"batch_owner"}))
        self.assertEqual(caught.exception.code, "UNKNOWN_ACTION")

    def test_covers_the_eleven_action_groups_of_section_9(self):
        self.assertGreaterEqual(len(ACTIONS), 25)


class FactsAreTheTrustBoundary(unittest.TestCase):
    """Blocker P1-02. A dict of booleans from a request body and a dict from
    the database look identical to a function signature, so the type is the
    boundary rather than a convention."""

    def test_a_plain_dict_is_refused(self):
        with self.assertRaises(PermissionError_) as caught:
            can("publish_batch", {"owns_batch": True})
        self.assertEqual(caught.exception.code, "UNTYPED_FACTS")

    def test_facts_without_provenance_are_refused(self):
        """An unattributed fact cannot be audited later, and the whole point of
        one permission table is that every decision can be explained."""
        with self.assertRaises(PermissionError_) as caught:
            AuthorizationFacts(
                actor_id="u1", roles=frozenset({"member"}), resource_id="r1"
            )
        self.assertEqual(caught.exception.code, "FACTS_WITHOUT_PROVENANCE")

    def test_anonymous_actor_is_refused(self):
        with self.assertRaises(PermissionError_) as caught:
            AuthorizationFacts(
                actor_id="", roles=frozenset(), resource_id=None, provenance="x"
            )
        self.assertEqual(caught.exception.code, "ANONYMOUS_ACTOR")

    def test_unknown_role_is_refused(self):
        with self.assertRaises(PermissionError_) as caught:
            AuthorizationFacts(
                actor_id="u1",
                roles=frozenset({"superuser"}),
                resource_id=None,
                provenance="x",
            )
        self.assertEqual(caught.exception.code, "UNKNOWN_ROLE")


class BatchOwnerIsNotOmnipotent(unittest.TestCase):
    """Section 9.1 lists what the batch owner may NOT do alone."""

    def test_may_freeze_and_publish_own_batch(self):
        proven = ["owns_batch", "all_recipients_eligible"]
        self.assertTrue(can("freeze_batch", facts({"batch_owner"}, proven)))
        self.assertTrue(can("publish_batch", facts({"batch_owner"}, proven)))

    def test_owning_the_batch_is_now_the_whole_of_publishing(self):
        """`all_recipients_eligible` went with the payment rail.

        It meant "every recipient has a usable bank account". There are no
        accounts, so the predicate could only ever be true, and a requirement
        that cannot fail is a line that reads like protection.
        """
        self.assertTrue(can("publish_batch", facts({"batch_owner"}, ["owns_batch"])))
        self.assertEqual(
            denial_reason("publish_batch", facts({"batch_owner"})),
            "owns_batch",
        )

    def test_may_not_freeze_someone_elses_batch(self):
        self.assertEqual(
            denial_reason("freeze_batch", facts({"batch_owner"})), "owns_batch"
        )

    def test_may_not_cancel_an_obligation_alone(self):
        self.assertEqual(
            denial_reason("cancel_obligation", facts({"batch_owner"})),
            "all_affected_parties_consented",
        )

    def test_nobody_may_delete_payment_or_receipt_events(self):
        """Deleting these would let the ledger be rewritten to suit whoever
        holds the button."""
        for action in (
            "delete_payment_report",
            "delete_receipt_confirmation",
            "delete_audit_history",
        ):
            for roles in ({"batch_owner"}, {"group_admin"}, {"platform_moderator"}):
                with self.subTest(action=action, roles=roles):
                    self.assertEqual(
                        denial_reason(action, facts(roles)),
                        "action_permitted_to_nobody",
                    )


class GroupAdminIsLogisticsNotFinance(unittest.TestCase):
    """Section 9.2."""

    def test_can_do_membership_logistics(self):
        for action in (
            "manage_members_and_invites",
            "remove_member_from_group",
            "transfer_group_admin",
        ):
            with self.subTest(action=action):
                self.assertTrue(can(action, facts({"group_admin"})))

    def test_cannot_adjudicate_identity(self):
        """In a group identity dispute the attacker is a group member, so the
        group cannot be the judge."""
        self.assertEqual(
            denial_reason("adjudicate_person_stub_claim", facts({"group_admin"})),
            "role_not_permitted",
        )

    def test_cannot_remove_other_peoples_content(self):
        self.assertEqual(
            denial_reason("remove_others_content", facts({"group_admin"})),
            "role_not_permitted",
        )


class WaiverBelongsToTheCreditor(unittest.TestCase):
    def test_organiser_cannot_forgive_on_someone_elses_behalf(self):
        self.assertEqual(
            denial_reason("waive_obligation", facts({"batch_owner", "member"})),
            "role_not_permitted",
        )

    def test_creditor_of_this_receivable_can(self):
        self.assertTrue(
            can(
                "waive_obligation",
                facts({"creditor"}, ["is_creditor_of_this_obligation"]),
            )
        )

    def test_creditor_of_a_different_receivable_cannot(self):
        self.assertEqual(
            denial_reason("waive_obligation", facts({"creditor"})),
            "is_creditor_of_this_obligation",
        )


class RevocationFollowsRisk(unittest.TestCase):
    """Section 9.1: whoever holds data or risk in a capability may pull it back."""

    def test_two_subjects_two_scopes(self):
        """There were three. The recipient-account scope had no account left.

        `revoke_capability_own_recipient_account` let a recipient pull back a
        capability because it carried THEIR bank details. With no bank details
        in an envelope there is no such risk and no such subject, so the entry
        is gone rather than left declaring a scope nobody can be in.
        """
        self.assertTrue(
            can("revoke_capability_whole_batch", facts({"batch_owner"}, ["owns_batch"]))
        )
        self.assertTrue(
            can(
                "revoke_capability_own_envelope",
                facts({"sender"}, ["is_own_capability"]),
            )
        )

    def test_a_sender_cannot_revoke_the_whole_batch(self):
        self.assertEqual(
            denial_reason(
                "revoke_capability_whole_batch", facts({"sender"}, ["owns_batch"])
            ),
            "role_not_permitted",
        )


class AdvancerAcknowledgement(unittest.TestCase):
    def test_only_the_named_advancer_may_acknowledge(self):
        """Gate 2 of section 8.3. Otherwise a member raises collections in
        someone else's name."""
        self.assertEqual(
            denial_reason("acknowledge_advancer_role", facts({"advancer"})),
            "is_named_advancer",
        )
        self.assertTrue(
            can("acknowledge_advancer_role", facts({"advancer"}, ["is_named_advancer"]))
        )


if __name__ == "__main__":
    unittest.main()


class FriendGraph(unittest.TestCase):
    """F03 and F04 entries, asserted as DATA.

    These exist because of a measured gap. Mutating each consent layer on its
    own showed that deleting `is_invitee` from `respond_to_friend_request`
    broke NOTHING: the domain state machine still refused, so every test
    stayed green while the permission table had quietly stopped guarding
    anything. A rule nobody checks is a rule that is already gone -- it just
    has not been noticed. The table is data, so the test reads the data.
    """

    def test_answering_a_request_requires_being_the_one_asked(self):
        self.assertEqual(
            denial_reason("respond_to_friend_request", facts({"member"})),
            "is_invitee",
        )

    def test_the_person_who_was_asked_may_answer(self):
        self.assertTrue(
            can("respond_to_friend_request", facts({"member"}, {"is_invitee"}))
        )

    def test_asking_requires_not_asking_yourself(self):
        self.assertEqual(
            denial_reason("send_friend_request", facts({"member"})), "is_not_self"
        )

    def test_anybody_may_ask_somebody_else(self):
        self.assertTrue(can("send_friend_request", facts({"member"}, {"is_not_self"})))

    def test_a_friend_list_belongs_to_its_owner(self):
        self.assertEqual(
            denial_reason("view_own_friends", facts({"member"})), "is_self"
        )
        self.assertTrue(can("view_own_friends", facts({"member"}, {"is_self"})))

    def test_a_guest_may_not_touch_the_friend_graph(self):
        """A capability token proves possession of one envelope, never identity."""
        for action in (
            "send_friend_request",
            "respond_to_friend_request",
            "view_own_friends",
            "find_person_by_phone",
        ):
            with self.subTest(action=action):
                self.assertEqual(
                    denial_reason(
                        action,
                        facts({"guest"}, {"is_invitee", "is_not_self", "is_self"}),
                    ),
                    "role_not_permitted",
                )


class TestPairNotebookDoors(unittest.TestCase):
    """Mười tám cửa của sổ hai người (ADR-0027). Mỗi cửa: đúng vai `member`,
    và không cửa nào mở khi thiếu vị từ của nó."""

    CUA = (
        "view_pair_notebook",
        "propose_pair_consent",
        "grant_pair_consent",
        "revoke_pair_consent",
        "draft_pair_paper",
        "view_pair_paper",
        "edit_pair_draft",
        "send_pair_paper",
        "view_pair_paper_as_recipient",
        "respond_pair_paper",
        "withdraw_pair_paper",
        "skip_pair_week",
        "record_pair_outing_done",
        "keep_pair_paper_line",
        "view_pair_constraints",
        "edit_pair_constraint",
        "preview_close_pair_notebook",
        "close_pair_notebook",
    )

    def facts(self, *proven: str) -> permissions.AuthorizationFacts:
        return permissions.AuthorizationFacts(
            actor_id="a",
            roles=frozenset({"member"}),
            resource_id="so-1",
            proven=frozenset(proven),
            provenance="test",
        )

    def test_muoi_tam_cua_deu_co_trong_bang(self):
        for action in self.CUA:
            self.assertIn(action, permissions.ACTIONS, action)
        self.assertEqual(len(set(self.CUA)), 18)

    def test_khong_cua_nao_dung_vai_ngoai_member(self):
        """Sổ hai người không có quản trị viên: không vai nào đủ một mình."""
        for action in self.CUA:
            self.assertEqual(permissions._TABLE[action]["roles"], {"member"}, action)
            self.assertTrue(permissions._TABLE[action]["requires"], action)

    def test_thieu_bat_ky_vi_tu_nao_la_dong(self):
        for action in self.CUA:
            requires = permissions._TABLE[action]["requires"]
            self.assertTrue(permissions.can(action, self.facts(*requires)), action)
            for bo_qua in requires:
                thieu = tuple(v for v in requires if v != bo_qua)
                self.assertEqual(
                    permissions.denial_reason(action, self.facts(*thieu)),
                    bo_qua,
                    f"{action} mở dù thiếu {bo_qua}",
                )

    def test_nguoi_ngoai_so_khong_mo_duoc_cua_nao(self):
        khach = permissions.AuthorizationFacts(
            actor_id="z",
            roles=frozenset({"guest"}),
            resource_id="so-1",
            proven=frozenset(
                {
                    "is_group_member",
                    "is_self",
                    "is_invitee",
                    "proposal_in_force",
                    "cycle_active_or_temporary",
                    "may_view_paper",
                    "is_draft_owner",
                    "version_current",
                    "is_not_version_sender",
                    "paper_unseen_unanswered",
                }
            ),
            provenance="test",
        )
        for action in self.CUA:
            self.assertEqual(
                permissions.denial_reason(action, khach), "role_not_permitted", action
            )

    def test_doc_to_giay_hep_hon_la_o_trong_so(self):
        """§3.3 luật 1: bản nháp chỉ chủ thấy, nên cửa đọc tờ KHÔNG nhận
        `is_group_member` làm đủ."""
        chi_thanh_vien = self.facts("is_group_member")
        self.assertEqual(
            permissions.denial_reason("view_pair_paper", chi_thanh_vien),
            "may_view_paper",
        )

    def test_tra_loi_khong_the_la_nguoi_gui_phien_ban_do(self):
        self.assertEqual(
            permissions.denial_reason(
                "respond_pair_paper", self.facts("is_group_member", "version_current")
            ),
            "is_not_version_sender",
        )
