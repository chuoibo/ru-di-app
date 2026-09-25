"""One permission table. Every API and every ActionItem asks this module.

Spec section 9 opens with the reason: scattered permission checks are exactly
how the confused deputy comes back. If three call sites each decide who may
publish a batch, they will disagree eventually, and the disagreement will be
discovered by someone collecting money they had no right to collect.

So the rules live here as data, and `can()` is the only way to read them.

Pure functions over plain dicts. No I/O, no ORM, no framework.
"""

from __future__ import annotations

from dataclasses import dataclass, field

__all__ = [
    "ACTIONS",
    "ROLES",
    "AuthorizationFacts",
    "can",
    "denial_reason",
    "PermissionError_",
]

ROLES = (
    "group_admin",
    "batch_owner",
    "advancer",
    "recipient",
    "sender",
    "creditor",
    "member",
    "former_member",
    "guest",
    "platform_moderator",
)

# Each entry is (roles that may act, extra predicates the context must satisfy).
# A role listed here is necessary, never sufficient on its own -- the predicates
# are what stop a batch_owner from acting outside their own batch.
_TABLE: dict[str, dict] = {
    # --- invocation ----------------------------------------------------
    "create_private_invocation": {"roles": {"member"}, "requires": ()},
    "create_shared_invocation": {"roles": {"member"}, "requires": ()},
    "view_invocation_input": {"roles": {"member"}, "requires": ("is_invoker",)},
    "view_invocation_proposal": {"roles": {"member"}, "requires": ("is_invoker",)},
    # --- expense -------------------------------------------------------
    "confirm_expense_proposal": {"roles": {"member"}, "requires": ("is_group_member",)},
    # Gate 2 of section 8.3. Only the person who actually fronted the money may
    # acknowledge that they fronted it; otherwise a member could raise
    # collections in someone else's name.
    "acknowledge_advancer_role": {
        "roles": {"advancer"},
        "requires": ("is_named_advancer",),
    },
    # --- collection board ----------------------------------------------
    # Reading who owes what, to whom, how much, and why somebody objected. The
    # endpoint shipped without this entry and without the check that uses it:
    # the service accepted an `actor` argument and never read it, so any valid
    # actor header plus a batch id returned every sender, every amount, and the
    # private reason a guest gave for disputing. Section 10 says visibility is
    # fail-closed; an unused parameter is the most convincing way to look like
    # it is while it is not.
    "view_collection_board": {"roles": {"member"}, "requires": ("is_group_member",)},
    # --- batch ---------------------------------------------------------
    "create_batch": {"roles": {"member"}, "requires": ("is_group_member",)},
    "freeze_batch": {"roles": {"batch_owner"}, "requires": ("owns_batch",)},
    "publish_batch": {
        "roles": {"batch_owner"},
        # `all_recipients_eligible` is gone with the payment rail: it meant
        # "every recipient has a usable bank account", and there are no
        # accounts now. Ownership is the whole of the check.
        "requires": ("owns_batch",),
    },
    # Section 9.1: whoever has data or risk inside a capability may pull it
    # back. Three different subjects, three different scopes.
    "revoke_capability_whole_batch": {
        "roles": {"batch_owner"},
        "requires": ("owns_batch",),
    },
    "revoke_capability_own_envelope": {
        "roles": {"sender"},
        "requires": ("is_own_capability",),
    },
    # --- guest settlement ----------------------------------------------
    # A bearer token is a capability, not proof of identity. The repository
    # first resolves it to one immutable envelope; these predicates ensure the
    # API never widens that scope while deciding what the holder may do.
    "view_guest_envelope": {"roles": {"guest"}, "requires": ("is_own_capability",)},
    "report_payment": {
        "roles": {"guest"},
        "requires": (
            "is_own_capability",
            "active_capability",
            "report_budget_available",
        ),
    },
    # Receipt confirmation is a financial event. Only the creditor of this
    # exact directed edge may create it; being a batch owner is irrelevant.
    "confirm_receipt": {
        "roles": {"recipient"},
        "requires": ("is_recipient_of_this_obligation",),
    },
    # --- things the batch owner may NOT do alone ------------------------
    "cancel_obligation": {
        "roles": {"batch_owner"},
        "requires": ("all_affected_parties_consented",),
    },
    "amend_obligation_after_publish": {
        "roles": {"batch_owner"},
        "requires": ("all_affected_parties_consented",),
    },
    "delete_payment_report": {"roles": set(), "requires": ()},
    "delete_receipt_confirmation": {"roles": set(), "requires": ()},
    "delete_audit_history": {"roles": set(), "requires": ()},
    "close_dispute": {"roles": {"platform_moderator"}, "requires": ()},
    # --- debt forgiveness ----------------------------------------------
    # Spec section 4: only the creditor of that exact receivable. The organiser
    # does not get to forgive on Ha's behalf.
    "waive_obligation": {
        "roles": {"creditor"},
        "requires": ("is_creditor_of_this_obligation",),
    },
    # --- evidence ------------------------------------------------------
    "request_redacted_evidence": {
        "roles": {"member", "guest"},
        "requires": ("is_charged_party",),
    },
    "share_evidence": {"roles": {"member"}, "requires": ("is_uploader",)},
    # --- identity ------------------------------------------------------
    # Naming somebody who has no row yet is how a name enters this product at
    # all: nobody signs up before a friend adds them to a dinner, so the
    # organiser types "Quyên" on their own phone. Section 7.2 calls the result
    # a PersonStub -- a name a member asserted, never proof of who that is.
    "register_person_identity": {"roles": {"group_admin", "member"}, "requires": ()},
    # Changing a name that already exists is a different act, and only the
    # person themselves may do it. A display name is what a stranger reads on
    # a guest page while deciding whether to send money; letting any member
    # rewrite it lets one member change who the page appears to be from.
    "rename_person_identity": {
        "roles": {"group_admin", "member"},
        "requires": ("is_self",),
    },
    # A guessed person id never makes a face public. Reading an avatar requires
    # an ACTIVE group shared with its subject; the self-case is trivially shared.
    "set_own_avatar": {
        "roles": {"group_admin", "member"},
        "requires": ("is_self",),
    },
    "view_person_avatar": {
        "roles": {"group_admin", "member"},
        "requires": ("shares_a_group_with_subject",),
    },
    "invite_person_stub_claim": {"roles": {"member"}, "requires": ()},
    "challenge_person_stub_claim": {"roles": {"member"}, "requires": ()},
    # Section 9.2: an admin does not adjudicate identity. Only the platform
    # does -- because in a group dispute the attacker is a group member.
    "adjudicate_person_stub_claim": {"roles": {"platform_moderator"}, "requires": ()},
    # --- friend graph (F03, F04) ----------------------------------------
    # Asking is not adding. `send_friend_request` creates a PENDING edge that
    # grants nothing, which is why its only predicate is that the actor is not
    # asking themselves; anyone with an account may ask anyone.
    #
    # `is_not_self` rather than a new predicate: the fact is identical to the
    # one `approve_link_join_request` already proves, and the note there is the
    # reason to reuse rather than mint. A second name for the same fact is a
    # second place to get it wrong.
    "send_friend_request": {"roles": {"member"}, "requires": ("is_not_self",)},
    # The consent gate. `is_invitee` is reused deliberately and exactly:
    # `accept_context_membership` already means "the person this was addressed
    # to may consent to it", and a friend request is the same shape -- an offer
    # aimed at one named person. Inventing `is_addressee` would have created
    # two predicates that must agree forever, which is what #128 cost us.
    #
    # Blocking is answered by this action too, and blocking is the one answer
    # either party may give. The extra latitude is proven in the service from
    # the row, not widened here: a permission table that says "either party"
    # would also let a requester accept.
    "respond_to_friend_request": {"roles": {"member"}, "requires": ("is_invitee",)},
    # Reading your own graph. `is_self` keeps one member from listing another
    # member's friends -- the social graph is the person's, not the group's.
    "view_own_friends": {"roles": {"member"}, "requires": ("is_self",)},
    # A person's own conversation list. `member` is granted to every signed-in
    # person before any membership exists (see `actor_grants`), so a brand-new
    # OTP account may ask and be told "no groups yet" rather than 403.
    "view_own_contexts": {"roles": {"member"}, "requires": ("is_self",)},
    # --- profile and bookmarks (M2) ---------------------------------------
    # Your own profile: read and edit are `is_self`, like the friend list.
    "view_own_profile": {"roles": {"member"}, "requires": ("is_self",)},
    "edit_own_profile": {"roles": {"member"}, "requires": ("is_self",)},
    # Somebody else's profile is visible to a friend or to a person who shares
    # an ACTIVE group with them, and to nobody else. The service proves
    # `is_visible_person` from the friend graph and the roster. An id that does
    # not exist and an id that is merely not visible get the SAME 403, so the
    # route is not an oracle for which ids exist.
    "view_person_profile": {
        "roles": {"member"},
        "requires": ("is_visible_person",),
    },
    # Bookmarks are the person's, not the group's.
    "manage_saved_places": {"roles": {"member"}, "requires": ("is_self",)},
    # Taste and budget are the person's too (M11, ADR-0019), and `is_self` is
    # the whole rule: there is no action here for reading somebody else's,
    # because no route hands one person another person's answers. A group only
    # ever sees them summed.
    "manage_own_interests": {"roles": {"member"}, "requires": ("is_self",)},
    # Resolving a telephone number the caller already holds to a person id.
    # No predicate beyond membership of the product, because the caller is
    # asking about a number they typed. What keeps this from being a directory
    # is that it answers with an id and a display name and never with a
    # telephone number -- see `routes/friends.py`, which is where that is
    # enforced and tested.
    "find_person_by_phone": {"roles": {"member"}, "requires": ()},
    # ADR-0023. Everything here is about the caller's own account, so the
    # predicate is `is_self` -- except blocking, where the fact that matters
    # is «not yourself», and lifting a block, where only the person who put it
    # up may take it down (`is_blocker`, proved from `decided_by_id`).
    "manage_own_sessions": {"roles": {"member"}, "requires": ("is_self",)},
    "delete_own_account": {"roles": {"member"}, "requires": ("is_self",)},
    "block_person": {"roles": {"member"}, "requires": ("is_not_self",)},
    "unblock_person": {"roles": {"member"}, "requires": ("is_blocker",)},
    "view_own_blocks": {"roles": {"member"}, "requires": ("is_self",)},
    "file_report": {"roles": {"member"}, "requires": ()},
    # --- group logistics ------------------------------------------------
    # These four actions require group membership, not outing ownership: the
    # trip belongs to the group, so any member may adjust its plan.
    "create_outing": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_outings": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "edit_outing_timeline": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "invite_to_outing": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F46. Arriving somewhere with the group is a group fact, so the gate is
    # the same ACTIVE membership the rest of this block uses: `is_group_member`
    # is satisfied only by an ACTIVE row, which is why an INVITED link holder
    # can neither record an arrival nor read who else has arrived.
    "check_in_to_stop": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_stop_checkins": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # Revocation is a group decision, so ACTIVE membership is the gate; an
    # INVITED link holder fails is_group_member.
    "revoke_outing_invite": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F17. Voting is a group activity, so the gate is the same ACTIVE
    # membership the block above uses. Closing is narrower: only the member who
    # opened the vote may end it, so nobody can cut short a poll they are
    # losing.
    "create_vote": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_votes": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "cast_vote_ballot": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "close_vote": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member", "is_vote_creator"),
    },
    "create_context": {"roles": {"group_admin", "member"}, "requires": ()},
    "invite_context_member": {
        "roles": {"group_admin"},
        "requires": ("is_group_member",),
    },
    "accept_context_membership": {
        "roles": {"group_admin", "member"},
        "requires": ("is_invitee",),
    },
    # Approval must come from somebody who is currently ACTIVE in the group and
    # who is not the requester. `is_group_member` is what actually refuses the
    # escalation today: a link redeemer holds an INVITED row, and INVITED is not
    # ACTIVE, so they fail the first predicate before the second is consulted.
    #
    # `is_not_self` is therefore redundant right now, and honestly so: deleting
    # it from this tuple breaks no test, because the partial unique index
    # `uq_memberships_open_per_person` makes the state it guards unreachable --
    # one person cannot hold both an ACTIVE row and an open INVITED row in the
    # same group. It is kept as the predicate that would still stand if that
    # index were ever relaxed, which is exactly the assumption rd-be-08 made
    # about `is_invitee` and got wrong. Do not read it as tested.
    "approve_link_join_request": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member", "is_not_self"),
    },
    "leave_context": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member", "is_self"),
    },
    "view_context_members": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "post_group_message": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_group_messages": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "invoke_group_companion": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # ADR-0021 §2.3. Taking a message back is the author's act and nobody
    # else's -- not even a group admin's. `is_author` is proved by the service
    # from the stored row, never from a body field naming a writer.
    "delete_own_message": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member", "is_author"),
    },
    # ADR-0021 §2.4. Renaming a group or choosing its theme is something any
    # active member may do, the way a messenger lets any participant retitle
    # a conversation. Roster changes stay behind `set_member_role`.
    "edit_context": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # ADR-0021 §2.5. Opening a private conversation with somebody needs one
    # fact only, proved from `friend_requests` at the moment of the call:
    # the two are friends. Friendship is the consent step; there is no
    # «accept conversation». The service turns this 403 into the one 404 every
    # refusal of the door shares, so the door is not an oracle for who is
    # friends with whom.
    "open_direct_message": {"roles": {"member"}, "requires": ("is_friend",)},
    # F32. A proactive suggestion is built from this group's own history --
    # where they went, what it cost, what kind of place they keep choosing --
    # so reading one is reading the group's past. Same ACTIVE gate as the
    # memory wall it is derived from: `is_group_member` is satisfied only by an
    # ACTIVE row, so an INVITED link holder cannot pull a group's history out
    # through a card that was never addressed to them.
    "view_group_suggestion": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F31. The implicit profile is the most concentrated thing the product
    # knows about a group: what they eat, what they do, and what they spend,
    # in one screen. It is derived from exactly the rows `view_group_memories`
    # guards, so it reuses that predicate rather than minting a softer one --
    # a profile is not "less private than the check-ins it was computed from"
    # merely because it arrives as scores instead of rows.
    "view_group_preference_profile": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F33. Reading this card means the server read the group's last few
    # messages. The gate is therefore the message gate, not a weaker one:
    # anyone who may not read the conversation may not read a card built out
    # of it either.
    "view_contextual_suggestion": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F36. An album is a way of reading photographs that already exist, so its
    # gate is deliberately the identical predicate the photo route uses. A
    # looser one here would make the album a way around that route -- which is
    # the whole failure mode a "collection" feature invites.
    "view_trip_album": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F43, F44, F45. All three read or answer about where the group goes, and
    # all three reuse `is_group_member` rather than minting a predicate: the
    # fact needed is identical to the one `view_group_memories` proves, because
    # the map and the heatmap are aggregations of exactly those rows. #128 was
    # the cost of two predicate names for one fact, and a "may_see_locations"
    # here would be that mistake with a location attached.
    #
    # Three actions rather than one, though, because they have different
    # subjects: two read the group's own history, and the third reads none of
    # it. Keeping them separate is what lets the map be withdrawn later without
    # also withdrawing a feature that never touched history.
    "view_social_map": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_group_heatmap": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # Meet-in-the-middle reads no stored location at all -- the caller supplies
    # unlabelled areas; see `app/places/meeting.py`. The gate is still ACTIVE
    # membership, because the answer is scored against the group's profile and a
    # former member should not keep a working group-planning endpoint.
    "view_meeting_point": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F34 carries the group's historical and current ledger totals. A context
    # id from a link is not authority to read them; only an ACTIVE row is.
    "view_group_budget": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # A reaction is a message-sized write by a member; reading them rides on
    # `view_group_messages`, which is why the list carries counts, not names.
    "react_to_message": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "post_group_memory": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    "view_group_memories": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # F39. Writing a post at all. Deliberately has no `is_group_member`: three
    # of the four F42 audiences address people rather than a group, and
    # requiring group membership to write an `only_me` note would make the
    # narrowest level the one hardest to reach.
    # ADR-0022 §2.2. Reading a post is proved by `post_audience.can_read`
    # (the 404 gate, run before any of these); reacting needs nothing more.
    # Commenting folds the wall owner's policy into `may_comment` through
    # `post_audience.can_comment`; deleting a comment is for its author or the
    # post's author (`can_delete_comment`).
    "react_to_post": {
        "roles": {"group_admin", "member"},
        "requires": ("may_read_post",),
    },
    "comment_on_post": {
        "roles": {"group_admin", "member"},
        "requires": ("may_comment",),
    },
    "delete_post_comment": {
        "roles": {"group_admin", "member"},
        "requires": ("may_delete_comment",),
    },
    # ADR-0022 §2.1. A personal photograph is uploaded only by its owner and
    # read by the owner or by somebody who may read a post that shows it --
    # the service proves `is_photo_addressee` from the posts table.
    "upload_personal_photo": {
        "roles": {"group_admin", "member"},
        "requires": ("is_self",),
    },
    "view_person_photo": {
        "roles": {"group_admin", "member"},
        "requires": ("is_photo_addressee",),
    },
    # ADR-0022 §2.3. A story is written by its author only; viewing is proved
    # by `story_visibility.can_view` (the 404 gate, run before `view_story`);
    # taking one down early is the author's alone.
    "create_story": {
        "roles": {"group_admin", "member"},
        "requires": ("is_self",),
    },
    "view_story": {
        "roles": {"group_admin", "member"},
        "requires": ("may_view_story",),
    },
    "delete_own_story": {
        "roles": {"group_admin", "member"},
        "requires": ("is_author",),
    },
    "create_post": {"roles": {"group_admin", "member"}, "requires": ()},
    # F42, and the only audience that needs a second permission. `create_post`
    # says the actor may write; this says they may point that writing at *this*
    # group. Two actions rather than one action with a conditional predicate,
    # because a predicate that is only sometimes required is a predicate
    # somebody eventually forgets to prove -- and here the thing forgotten
    # would be the roster check that keeps a stranger from posting into a
    # group's feed. Reuses `is_group_member`, the same fact
    # `post_group_memory` proves, rather than minting a name for it.
    "address_post_to_group": {
        "roles": {"group_admin", "member"},
        "requires": ("is_group_member",),
    },
    # Administration is scoped to one group, so `is_group_admin` must come
    # from that group's active membership row. X-Actor-Roles cannot prove
    # which group its broad `group_admin` claim applies to.
    "set_member_role": {
        "roles": {"group_admin"},
        "requires": ("is_group_admin",),
    },
    "manage_members_and_invites": {"roles": {"group_admin"}, "requires": ()},
    "remove_member_from_group": {"roles": {"group_admin"}, "requires": ()},
    "transfer_group_admin": {"roles": {"group_admin"}, "requires": ()},
    "remove_own_uploaded_content": {
        "roles": {"group_admin", "member"},
        "requires": ("is_uploader",),
    },
    "remove_others_content": {"roles": {"platform_moderator"}, "requires": ()},
    "attach_workspace_to_group": {
        "roles": {"member"},
        "requires": ("is_workspace_owner",),
    },
    # --- sổ hai người và tờ giấy (ADR-0027) -----------------------------
    # Eighteen doors for one notebook, and every one of them `member` plus a
    # predicate: nothing here is a role. A notebook belongs to exactly two
    # people, so «who you are» is never enough and «which notebook» is always
    # the question.
    #
    # `is_group_member` is reused deliberately and exactly. A pair IS a context
    # with two ACTIVE memberships (ADR-0021 §2.5), and `post_group_message`
    # already proves the same fact for the same rows when somebody writes into
    # a private conversation. Minting `is_pair_member` would create two names
    # for one fact, which is what #128 cost this repository once already.
    #
    # ADR-0027 §4: Nếp is never an actor. There is no action here a non-person
    # could take, and the one sheet Nếp writes is written by the server on a
    # person's command (`draft_pair_paper`), which is why that door exists.
    "view_pair_notebook": {"roles": {"member"}, "requires": ("is_group_member",)},
    "propose_pair_consent": {"roles": {"member"}, "requires": ("is_group_member",)},
    # Granting is the other person's act. `is_invitee` is reused for the same
    # reason `respond_to_friend_request` reuses it: the fact is «this offer was
    # aimed at one named person and I am that person», and a consent proposal
    # is that shape. `proposal_in_force` is new because no offer in this
    # product had a deadline of its own before: a consent nobody answered
    # inside its window stops being an offer rather than waiting forever.
    "grant_pair_consent": {
        "roles": {"member"},
        "requires": ("is_invitee", "proposal_in_force"),
    },
    # Taking consent back is the grantor's act alone, and it is `is_self`
    # because the row names the person: one half of a pair cannot revoke the
    # other half's grant, which would let one person lock the other out of a
    # notebook they both agreed to.
    "revoke_pair_consent": {"roles": {"member"}, "requires": ("is_self",)},
    # Asking the notebook for a sheet. Either person may, at any time -- the
    # turn decides whose name Nếp drafts it FOR, not who may ask (§6.3). The
    # second predicate is what stops a sheet appearing in a notebook that was
    # never opened or has been closed; `is_temporary` covers the very first
    # invitation, which happens before a cycle exists (§14.1).
    "draft_pair_paper": {
        "roles": {"member"},
        "requires": ("is_group_member", "cycle_active_or_temporary"),
    },
    # §3.3 rule 1: a draft is not a sent sheet. The other person's machine does
    # not receive one, so reading is narrower than membership and
    # `may_view_paper` is proved from `draft_owner_id` for a `nhap` sheet.
    "view_pair_paper": {"roles": {"member"}, "requires": ("may_view_paper",)},
    "edit_pair_draft": {"roles": {"member"}, "requires": ("is_draft_owner",)},
    # Sending pins the version: a client that read v1, waited, and pressed send
    # after the notebook moved on must not send v1's content as v2.
    "send_pair_paper": {
        "roles": {"member"},
        "requires": ("is_draft_owner", "version_current"),
    },
    # The view mark (§7.5) is the recipient's, and the recipient is «whoever
    # did not send this version». One predicate for both this and answering,
    # because it is one fact.
    "view_pair_paper_as_recipient": {
        "roles": {"member"},
        "requires": ("is_not_version_sender",),
    },
    # Order matters here and only here. Both predicates can be missing at once
    # -- somebody answering a version they sent, which has since been replaced
    # -- and the table reports the first one. «The sheet has moved on» tells the
    # person to look again; «that is yours» tells them about a screen they are
    # no longer looking at. Nothing is hidden by the choice: the actor may read
    # the sheet either way.
    "respond_pair_paper": {
        "roles": {"member"},
        "requires": ("version_current", "is_not_version_sender"),
    },
    # ADR-0027: withdrawal is the sender's, and only while nothing has come
    # back. `paper_unseen_unanswered` is proved from the view and response
    # rows, never from a flag a client sent.
    "withdraw_pair_paper": {
        "roles": {"member"},
        "requires": ("is_group_member", "paper_unseen_unanswered"),
    },
    # «Tuần này nghỉ», recording that the outing happened, and keeping a line
    # afterwards are all things either person may do: they are facts about a
    # week the two of them share, not about who wrote what.
    "skip_pair_week": {"roles": {"member"}, "requires": ("is_group_member",)},
    "record_pair_outing_done": {"roles": {"member"}, "requires": ("is_group_member",)},
    "keep_pair_paper_line": {"roles": {"member"}, "requires": ("is_group_member",)},
    # §6.4: the two constraints live in the shared area, so both may read them;
    # only their owner may write one, because a constraint is something its
    # owner says about themselves.
    "view_pair_constraints": {"roles": {"member"}, "requires": ("is_group_member",)},
    "edit_pair_constraint": {"roles": {"member"}, "requires": ("is_self",)},
    # ADR-0034 §2.4: either of the two may say who leads this week; it decides
    # whose turn the week reads as and grants nothing.
    "set_pair_week_role": {"roles": {"member"}, "requires": ("is_group_member",)},
    # Closing is a two-step door on purpose (§7.6): the preview is a read that
    # produces the revision the close must carry, so the count somebody agreed
    # to and the rows being closed are provably the same rows.
    "preview_close_pair_notebook": {
        "roles": {"member"},
        "requires": ("is_group_member",),
    },
    "close_pair_notebook": {"roles": {"member"}, "requires": ("is_group_member",)},
}

ACTIONS = tuple(sorted(_TABLE))


class PermissionError_(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class AuthorizationFacts:
    """What an authoritative source proved, not what a request claimed.

    The first version took a plain dict of booleans, so one adapter setting one
    flag wrongly bypassed the whole table -- exactly the confused deputy this
    module exists to prevent. A dict from an HTTP body and a dict from the
    database look identical to a function signature.

    So the type is the boundary. Only an adapter that read authoritative data
    can build this, `provenance` records which one did, and `can()` refuses
    anything else. Nothing here validates the facts; it makes the caller state
    where they came from, which is what an audit needs.
    """

    actor_id: str
    roles: frozenset[str]
    resource_id: str | None
    proven: frozenset[str] = field(default_factory=frozenset)
    provenance: str = ""

    def __post_init__(self):
        if not self.actor_id:
            raise PermissionError_("ANONYMOUS_ACTOR")
        if not self.provenance:
            # An unattributed fact cannot be audited later, and the point of
            # one permission table is that every decision can be explained.
            raise PermissionError_("FACTS_WITHOUT_PROVENANCE")
        unknown = set(self.roles) - set(ROLES)
        if unknown:
            raise PermissionError_("UNKNOWN_ROLE")


def can(action: str, facts: AuthorizationFacts) -> bool:
    """True when `facts` permit `action`."""
    return denial_reason(action, facts) is None


def denial_reason(action: str, facts: AuthorizationFacts) -> str | None:
    """None when allowed, otherwise the name of what is missing.

    Returning the reason rather than a bare False is deliberate: the interface
    has to tell somebody what is missing. "The advancer has not acknowledged
    yet" is actionable; "forbidden" is not.
    """
    if action not in _TABLE:
        raise PermissionError_("UNKNOWN_ACTION")
    if not isinstance(facts, AuthorizationFacts):
        # A plain dict is how a request body sneaks in wearing the costume of
        # a database read.
        raise PermissionError_("UNTYPED_FACTS")

    rule = _TABLE[action]
    if not rule["roles"]:
        # Nobody, ever. Deleting a receipt confirmation would let the ledger be
        # rewritten to suit whoever holds the button.
        return "action_permitted_to_nobody"
    if not (set(facts.roles) & rule["roles"]):
        return "role_not_permitted"
    for predicate in rule["requires"]:
        if predicate not in facts.proven:
            return predicate
    return None
