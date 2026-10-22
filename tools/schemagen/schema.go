package main

// appTables are the tables this application owns, moving into the board's own
// database. They were two SQLite files beside it —
// `acp.db` and `sources.db` — and everything they know is about a card or a
// board, which lives in a third file where no foreign key could reach it.
//
// What that cost is not hypothetical: deleting a card is a real DELETE FROM
// blocks, and nothing here ever heard about it, so every deleted card left its
// sessions, its position on a route, its stall and its queue slot behind
// for ever. The keys below are the fix, and they are written in CREATE TABLE
// because SQLite has no ALTER TABLE ADD CONSTRAINT — there is no second chance
// to add them.
//
// Two things are deliberately still as they were, and both are step 0's to
// change: ids are the board's 27-character strings rather than UUIDv7, and
// times are unix milliseconds rather than timestamps. Neither can move on its
// own — MySQL refuses a foreign key across differing column types, so our ids
// and the board's change in one migration or in none.
func appTables() []Table {
	return []Table{
		// What everything below points at instead of the board's own tables, so
		// a key still holds when the board is on somebody else's server.
		boardRef(),
		cardRef(),

		// The machine's registries. They were arrays in config.json, which is a
		// file nothing can point at: a card named its folder by an id the
		// settings file happened to carry, and a column named its agent by the
		// name a person typed.
		proxy(),
		workspace(),
		workspaceBoard(),
		agent(),

		agentSession(),
		sessionEvent(),
		idempotency(),
		cardStall(),
		cardChanges(),
		boardSetup(),
		checkout(),
		sourceItem(),
		sourceEvent(),
		boardSummary(),
		inboxSort(),
	}
}

// workspace is a named place where work can happen. It was `workdir`, and the
// rename is the point rather than decoration: nothing about the entry has to be
// a directory except its `path`, and the reason it has an id at all is that
// tomorrow it may be a repository to clone, a drive or a machine over ssh. The
// screen calls it a folder and will go on doing so.
//
// The git *settings* live here — kind, base branch, branch prefix, and the mode
// per board in workspace_board. The git *state* of one card's work is a
// checkout (below).
func workspace() Table {
	return Table{
		Name: "workspace",
		Why: "A named place an agent can work in. Was the `projects` array in\n" +
			"config.json; a card points at it by id, which is also the id of the\n" +
			"board option offered for it, so a card naming its folder is a card\n" +
			"holding an ordinary select value that happens to be a reference.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "name", Type: Name(200),
				Why: "A caption, not a key. Renaming it breaks nothing, which is the\n" +
					"whole reason the id exists."},
			{Name: "path", Type: Text(), Null: true},
			{Name: "board_id", Type: ID(), Null: true,
				Why: "The board that offers this folder, and the only one that does.\n" +
					"NULL means no board has claimed it — a state the product already\n" +
					"has a name and a screen for, which is why a deleted board sets this\n" +
					"to NULL rather than taking the registry entry with it."},
			{Name: "global", Type: Bool(),
				Why: "«На всех досках»: one entry seen from several boards, which is what\n" +
					"makes the mode belong to the pair rather than to the folder."},
			{Name: "kind", Type: Name(16), Null: true,
				Why: "What somebody was promised, not what the folder happens to be:\n" +
					"'git' means a repository was demanded and one without git is an\n" +
					"error. NULL is the ordinary case and means nobody said."},
			{Name: "base_branch", Type: Name(200), Null: true},
			{Name: "branch_prefix", Type: Name(64), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		FKs: []FK{{
			Name: "workspace_offered_on", Columns: []string{"board_id"},
			RefTable: tableBoards, RefCols: []string{"id"}, OnDelete: SetNull,
		}},
		Indexes: []Index{
			{Name: "idx_workspace_name", Columns: []string{"name"}, Unique: true},
		},
	}
}

// workspace_board is how a folder is worked in on one board.
func workspaceBoard() Table {
	return Table{
		Name: "workspace_board",
		Why: "Which board a folder is offered to and how work happens in it there.\n" +
			"Keyed by the pair rather than by the folder, because a folder marked\n" +
			"«на всех досках» is one entry with a different answer per board: a copy\n" +
			"per card where three people work it, a branch in place where one does.\n" +
			"Was WorkdirEntry.Modes, a map keyed by board id.",
		Columns: []Column{
			{Name: "workspace_id", Type: ID()},
			{Name: "board_id", Type: ID()},
			{Name: "mode", Type: Name(16), Null: true,
				Why: "NULL means nobody answered, and the machine's own default stands in."},
		},
		PK: []string{"workspace_id", "board_id"},
		Checks: []Check{{
			Name: "workspace_board_mode", Column: "mode",
			// Not 'plain': that is what an ordinary folder does, and a folder
			// nobody was asked about takes the machine's default (NULL).
			Values: []string{"worktree", "branch"},
		}},
		FKs: []FK{
			{Name: "workspace_board_workspace", Columns: []string{"workspace_id"},
				RefTable: "workspace", RefCols: []string{"id"}, OnDelete: Cascade},
			{Name: "workspace_board_board", Columns: []string{"board_id"},
				RefTable: tableBoards, RefCols: []string{"id"}, OnDelete: Cascade},
		},
	}
}

// proxy is one named network configuration.
func proxy() Table {
	return Table{
		Name: "proxy",
		Why: "Network settings several agents share, so they are edited in one\n" +
			"place. Was the `proxies` array in config.json.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "name", Type: Name(100)},
			{Name: "url", Type: Text(), Null: true},
			{Name: "no_proxy", Type: Text(), Null: true},
			{Name: "ca_cert", Type: Text(), Null: true},
			{Name: "username", Type: Name(100), Null: true},
			{Name: "password", Type: Text(), Null: true,
				Why: "Kept apart from the URL so it is entered raw and masked on screen.\n" +
					"Still stored in the clear here, exactly as it was in the settings\n" +
					"file: moving it into internal/secrets is its own change."},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		Indexes: []Index{
			{Name: "idx_proxy_name", Columns: []string{"name"}, Unique: true},
		},
	}
}

// agent is one registered coding agent.
func agent() Table {
	return Table{
		Name: "agent",
		Why: "A registered agent. Its board account is a row in users, and the two\n" +
			"are joined by a key rather than by their names happening to match —\n" +
			"which is what made renaming an agent break the crew of every route on\n" +
			"every board, silently.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "name", Type: Name(100),
				Why: "On screen and in the account's username. No longer a reference."},
			{Name: "kind", Type: Name(32)},
			{Name: "user_id", Type: ID(), Null: true,
				Why: "The board account. NULL until one is made, and NULL again if it is\n" +
					"deleted: the registry entry is the machine's and outlives it."},
			{Name: "owner_id", Type: ID(), Null: true,
				Why: "The person whose agent this is, and whose model key it runs on. NULL on\n" +
					"a desktop, where the one person there owns every agent."},
			{Name: "proxy_id", Type: ID(), Null: true},
			{Name: "bin_path", Type: Text(), Null: true},
			{Name: "model", Type: Name(100), Null: true},
			{Name: "prompt", Type: Text(), Null: true},
			{Name: "settings", Type: JSON(), Null: true,
				Why: "env, args, cliArgs, options, autoAllowTools, command,\n" +
					"mcpServers. JSON on purpose: this is how to start a process, it\n" +
					"points at nothing and nothing joins to it."},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		FKs: []FK{
			{Name: "agent_user", Columns: []string{"user_id"},
				RefTable: tableUsers, RefCols: []string{"id"}, OnDelete: SetNull},
			{Name: "agent_owner", Columns: []string{"owner_id"},
				RefTable: tableUsers, RefCols: []string{"id"}, OnDelete: Cascade},
			{Name: "agent_proxy", Columns: []string{"proxy_id"},
				RefTable: "proxy", RefCols: []string{"id"}, OnDelete: SetNull},
		},
		Indexes: []Index{
			{Name: "idx_agent_name", Columns: []string{"name"}, Unique: true},
		},
	}
}

// boardTables are the fork's own, named here only so a foreign key has
// something to point at. They are not emitted: migration 000001 made them.
const (
	tableBlocks = "blocks"
	tableBoards = "boards"
	tableUsers  = "users"
)

// boardRef and cardRef are what a foreign key can point at when the board is
// somebody else's.
//
// A key can only reach inside its own database. Everything below used to point
// straight at blocks(id) and boards(id), and that key is the whole reason these
// tables live in the board's database at all: deleting a card is a real DELETE
// FROM blocks, which this side never heard about, so sessions, route
// positions and stalls leaked for ever.
//
// A desktop attached to a hosted board has no such rows — a remote card is not
// in this machine's blocks — so the insert would not be ambiguous, it would be
// refused. These two tables are the row the key points at instead. The chain is
// blocks → card_ref → everything, so a local card cascades exactly as it did;
// a remote one carries no block_id and its ref row is deleted by the client
// that hears the board say the card is gone.
//
// Not a surrogate id: the ref is keyed by the card's own id, so not one query
// or column anywhere else had to change. Ids are UUIDv7 and two servers
// colliding on one is not a risk worth a join.
func boardRef() Table {
	return Table{
		Name: "board_ref",
		Why: "A board this machine knows about, whoever's it is. What the app's own\n" +
			"tables point at instead of boards(id), which only has the boards\n" +
			"served by this process.",
		Columns: []Column{
			{Name: "board_id", Type: ID(),
				Why: "The board's own id, on whichever server it lives."},
			{Name: "source_id", Type: Name(64), Null: true,
				Why: "Where the board is. NULL is this machine, which is every board\n" +
					"until a desktop attaches to a hosted one."},
			{Name: "local_board_id", Type: ID(), Null: true,
				Why: "The same board in this process's own table, when it is ours. NULL\n" +
					"for a board on a server: that is what makes the key possible at all."},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"board_id"},
		FKs: []FK{{
			Name: "board_ref_local", Columns: []string{"local_board_id"},
			RefTable: tableBoards, RefCols: []string{"id"}, OnDelete: Cascade,
		}},
	}
}

func cardRef() Table {
	return Table{
		Name: "card_ref",
		Why: "A card this machine knows about. The row every table keyed on a card\n" +
			"points at, so the cascade survives the board being somewhere else.",
		Columns: []Column{
			{Name: "card_id", Type: ID()},
			{Name: "board_id", Type: ID(), Null: true,
				Why: "Which board it is on, and through that which server. Nullable\n" +
					"because three of the writers know a card and not its board — a\n" +
					"stall, a flow event, a checkout — and the board is filled in by\n" +
					"whichever writer knows it."},
			{Name: "block_id", Type: ID(), Null: true,
				Why: "The same card in this process's own blocks, when it is ours. This\n" +
					"is the cascade: the card is deleted, the block goes, this row goes,\n" +
					"and everything about the card goes with it."},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"card_id"},
		FKs: []FK{
			{Name: "card_ref_board", Columns: []string{"board_id"},
				RefTable: "board_ref", RefCols: []string{"board_id"}, OnDelete: Cascade},
			{Name: "card_ref_block", Columns: []string{"block_id"},
				RefTable: tableBlocks, RefCols: []string{"id"}, OnDelete: Cascade},
		},
	}
}

// cardFK is the key almost every table here wants: what we know about a card
// stops being true the moment the card does.
func cardFK(table string) FK {
	return FK{
		Name:     table + "_card",
		Columns:  []string{"card_id"},
		RefTable: "card_ref",
		RefCols:  []string{"card_id"},
		OnDelete: Cascade,
	}
}

func boardFK(table string) FK {
	return FK{
		Name:     table + "_board",
		Columns:  []string{"board_id"},
		RefTable: "board_ref",
		RefCols:  []string{"board_id"},
		OnDelete: Cascade,
	}
}

// agent_session is a run over the protocol: a card's work, or the short
// headless run that names a branch. A planning conversation is
// not one: it is about no card and leaves no row.
func agentSession() Table {
	return Table{
		Name: "agent_session",
		Why:  "A run of an agent over ACP. One process, one turn, one verdict.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "card_id", Type: ID(), Null: true,
				Why: "NULL for a run that is about no card — naming a branch is one."},
			{Name: "board_id", Type: ID(), Null: true},
			{Name: "agent_kind", Type: Name(32)},
			{Name: "acp_session_id", Type: Name(100), Null: true},
			{Name: "status", Type: Name(24)},
			{Name: "column_key", Type: Name(128), Null: true,
				Why: "Which column's limit this run counts against. Here rather than in\n" +
					"memory because a column's limit is about the board and not about\n" +
					"one process: two windows of this app, and one day two machines on\n" +
					"one board, all have to be counting the same sessions."},
			{Name: "cwd", Type: Text(), Null: true},
			{Name: "worktree_path", Type: Text(), Null: true},
			{Name: "branch", Type: Name(255), Null: true},
			{Name: "started_at", Type: Millis()},
			{Name: "finished_at", Type: Millis(), Null: true,
				Why: "NULL means the session is still live."},
			{Name: "error_text", Type: Text(), Null: true},
		},
		PK: []string{"id"},
		Checks: []Check{{
			Name: "agent_session_status", Column: "status",
			// waiting_permission is a question open to a person; idle is no
			// longer reached — a session runs its task and ends — but a status
			// read back has to mean something (acp.SessionStatus).
			Values: []string{"queued", "running", "idle", "waiting_permission",
				"done", "failed", "cancelled"},
		}},
		FKs: []FK{cardFK("agent_session"), boardFK("agent_session")},
		Indexes: []Index{
			{Name: "idx_agent_session_card", Columns: []string{"card_id", "started_at"}},
		},
	}
}

// session_event is one session's journal.
func sessionEvent() Table {
	return Table{
		Name: "session_event",
		Why: "What happened during a session, in order. The id is a UUIDv7 rather\n" +
			"than an autoincrement: v7 sorts by time, so ORDER BY id still means\n" +
			"\"as it happened\" and the three spellings of AUTO_INCREMENT are gone.\n" +
			"`seq` went with them — it was the same fact told twice.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "session_id", Type: ID()},
			{Name: "kind", Type: Name(32)},
			{Name: "payload_json", Type: JSON(), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		FKs: []FK{{
			Name: "session_event_session", Columns: []string{"session_id"},
			RefTable: "agent_session", RefCols: []string{"id"}, OnDelete: Cascade,
		}},
		Indexes: []Index{
			{Name: "idx_session_event_session", Columns: []string{"session_id", "id"}},
		},
	}
}

// idempotency is the flow engine's latch: this event has already been handled.
func idempotency() Table {
	return Table{
		Name: "idempotency",
		Why: "One route event, handled once. `key` was the column's name and is a\n" +
			"reserved word in MySQL, so it is `token` — cheaper than backticks in\n" +
			"every query for ever.",
		Columns: []Column{
			{Name: "token", Type: Name(255)},
			{Name: "session_id", Type: ID(), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"token"},
		Indexes: []Index{
			{Name: "idx_idempotency_created", Columns: []string{"created_at"}},
		},
	}
}

// card_stall is why a card is standing still.
func cardStall() Table {
	return Table{
		Name: "card_stall",
		Why: "One current reason a card is not moving — not a journal. The reason is\n" +
			"true only until somebody fixes what it is about, and a comment would\n" +
			"outlive it as noise.",
		Columns: []Column{
			{Name: "card_id", Type: ID()},
			{Name: "node_id", Type: Name(64), Null: true},
			// reason is the English; the page words reason_code with the
			// values in reason_args (a JSON object), so the stall reads in the
			// language of whoever looks (docs/i18n.md).
			{Name: "reason", Type: Text()},
			{Name: "reason_code", Type: Name(64), Null: true},
			{Name: "reason_args", Type: Text(), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK:  []string{"card_id"},
		FKs: []FK{cardFK("card_stall")},
	}
}

// card_changes is the size of what a card's work amounts to.
func cardChanges() Table {
	return Table{
		Name: "card_changes",
		Why: "How much a card's branch holds, in numbers. The diff itself is read\n" +
			"from git every time and never from here — this is what the button\n" +
			"that opens it is drawn on, and what a board could show per card.\n" +
			"Recorded rather than counted when asked, for two reasons: a stage\n" +
			"ending folds the working copy away (FoldWorktree) while the question\n" +
			"is asked on the stage after that one, and a board asking it per card\n" +
			"is a git process per card.",
		Columns: []Column{
			{Name: "card_id", Type: ID()},
			{Name: "branch", Type: Name(255), Null: true},
			{Name: "base", Type: Name(255), Null: true,
				Why: "What the branch was cut from, so a stale row can be told from one\n" +
					"about a different base."},
			{Name: "files", Type: Int()},
			{Name: "added", Type: Int()},
			{Name: "removed", Type: Int()},
			{Name: "dirty", Type: Int(),
				Why: "Files changed and not committed. Counted apart because they exist\n" +
					"only while a working copy does, and the committed half survives it."},
			{Name: "computed_at", Type: Millis()},
		},
		PK:  []string{"card_id"},
		FKs: []FK{cardFK("card_changes")},
	}
}

// board_setup is what the setup wizard has already asked about a board.
func boardSetup() Table {
	return Table{
		Name: "board_setup",
		Why: "Which setup questions a board has been asked. It is about this machine\n" +
			"— the answers are folders and hosts here — so it cannot live in the\n" +
			"browser's storage, which the desktop window forgets on every launch.",
		Columns: []Column{
			{Name: "board_id", Type: ID()},
			{Name: "step", Type: Name(32)},
			{Name: "status", Type: Name(16)},
			{Name: "changed_at", Type: Millis(),
				Why: "Was `at`. AT is a keyword in Postgres and a name not worth having."},
		},
		PK: []string{"board_id", "step"},
		Checks: []Check{{
			Name: "board_setup_status", Column: "status",
			// 'offered' is the wizard's own row rather than a step's answer:
			// it records that a board was shown the wizard at all.
			Values: []string{"pending", "done", "skipped", "offered"},
		}},
		FKs: []FK{boardFK("board_setup")},
	}
}

// checkout is the git state of one owner's work in one workspace. It was
// workdir_claim, keyed by the folder's path and by an `owner` string that was
// either a card id or "board:<id>" — one column pointing at two tables, so no
// key was possible and a folder somebody moved orphaned every claim in it.
//
// The name describes what was already true rather than reinterpreting
// anything: an ordinary folder creates no row here at all, because
// ClaimWorkspace records nothing for WorkModePlain. This table has only ever
// held git copies.
func checkout() Table {
	return Table{
		Name: "checkout",
		Why: "The directory and the branch one owner holds in one workspace. The\n" +
			"owner is a card, or a board for a conversation with no card — two\n" +
			"nullable columns rather than one string, because a foreign key cannot\n" +
			"point at two tables through one.",
		Columns: []Column{
			{Name: "id", Type: ID(),
				Why: "A surrogate, and the one place in this schema where that is right:\n" +
					"the natural key would have to include a nullable column, which MySQL\n" +
					"and Postgres forbid in a primary key. Uniqueness is stated in the\n" +
					"indexes below instead, where a NULL is allowed to differ from a NULL.\n" +
					"Nothing outside this application creates a checkout, so no client has\n" +
					"to be able to supply the id."},
			{Name: "workspace_id", Type: ID()},
			{Name: "card_id", Type: ID(), Null: true},
			{Name: "board_id", Type: ID(), Null: true,
				Why: "Set for «черновики доски»: a conversation with no card still works\n" +
					"somewhere, and that somewhere belongs to the board."},
			{Name: "mode", Type: Name(16)},
			{Name: "branch", Type: Name(255), Null: true},
			{Name: "path", Type: Text(), Null: true,
				Why: "Where the copy is. Not the workspace's own path — that is on the\n" +
					"workspace, and this is the worktree cut from it."},
			{Name: "base", Type: Name(255), Null: true,
				Why: "What the branch was cut from, and therefore what «merged» means."},
			{Name: "created_at", Type: Millis()},
			{Name: "released_at", Type: Millis(), Null: true,
				Why: "NULL means the checkout is live."},
		},
		PK: []string{"id"},
		Checks: []Check{{
			Name:   "checkout_mode",
			Column: "mode",
			Values: []string{"worktree", "branch", "plain"},
		}},
		FKs: []FK{
			// A workspace a card is still working in cannot be deleted: there
			// is a copy on disk and it has to be folded away first.
			{Name: "checkout_workspace", Columns: []string{"workspace_id"},
				RefTable: "workspace", RefCols: []string{"id"}, OnDelete: Restrict},
			cardFK("checkout"),
			boardFK("checkout"),
		},
		Indexes: []Index{
			// One checkout per owner per workspace, said twice because the
			// owner is one column or the other. A NULL does not collide with a
			// NULL in any of the three dialects, which is what lets these two
			// say what a composite primary key could not.
			{Name: "idx_checkout_card", Columns: []string{"workspace_id", "card_id"}, Unique: true},
			{Name: "idx_checkout_board", Columns: []string{"workspace_id", "board_id"}, Unique: true},
			{Name: "idx_checkout_live", Columns: []string{"workspace_id", "released_at"}},
		},
	}
}

// source_item is the inbox's dedup.
func sourceItem() Table {
	return Table{
		Name: "source_item",
		Why: "What a source has already brought. The same letter arriving twice adds\n" +
			"to the card it made rather than making a second one.",
		Columns: []Column{
			{Name: "source", Type: Name(100)},
			{Name: "external_id", Type: Name(255)},
			{Name: "version", Type: Name(255), Null: true,
				Why: "The service's own updated/etag: changed means there is something\n" +
					"to add."},
			{Name: "card_id", Type: ID(), Null: true},
			{Name: "created_at", Type: Millis()},
			{Name: "updated_at", Type: Millis()},
		},
		PK: []string{"source", "external_id"},
		FKs: []FK{{
			Name: "source_item_card", Columns: []string{"card_id"},
			RefTable: "card_ref", RefCols: []string{"card_id"}, OnDelete: SetNull,
		}},
		Indexes: []Index{
			{Name: "idx_source_item_card", Columns: []string{"card_id"}},
		},
	}
}

// source_event is a source's log.
func sourceEvent() Table {
	return Table{
		Name: "source_event",
		Why: "Why nothing happened — the only question anybody asks of a source.\n" +
			"Deleting the card it produced does not delete the line: what a source\n" +
			"decided is a fact about the source.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "source", Type: Name(100)},
			{Name: "external_id", Type: Name(255), Null: true},
			{Name: "rule", Type: Name(100), Null: true},
			{Name: "outcome", Type: Name(16)},
			{Name: "card_id", Type: ID(), Null: true},
			// detail is the English; detail_code and detail_args (a JSON
			// object) are what the page words, for what this app wrote.
			{Name: "detail", Type: Text(), Null: true},
			{Name: "detail_code", Type: Name(64), Null: true},
			{Name: "detail_args", Type: Text(), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		Checks: []Check{{
			Name:   "source_event_outcome",
			Column: "outcome",
			Values: []string{"created", "commented", "inbox", "dropped", "failed", "skipped"},
		}},
		FKs: []FK{{
			Name: "source_event_card", Columns: []string{"card_id"},
			RefTable: "card_ref", RefCols: []string{"card_id"}, OnDelete: SetNull,
		}},
		Indexes: []Index{
			{Name: "idx_source_event_source", Columns: []string{"source", "id"}},
		},
	}
}

// board_summary is what a board's cards are about, in a few sentences.
func boardSummary() Table {
	return Table{
		Name: "board_summary",
		Why: "What a board's cards are about, written by an agent from their titles.\n" +
			"It is what the inbox is sorted by beside the board's own description:\n" +
			"a description says what a board was made for, and the cards say what\n" +
			"it is used for. Kept here rather than on the board because it is this\n" +
			"machine's reading of the board, redone whenever the cards change.",
		Columns: []Column{
			{Name: "board_id", Type: ID()},
			{Name: "summary", Type: Text()},
			{Name: "card_count", Type: Int()},
			{Name: "fingerprint", Type: Name(64),
				Why: "A hash of the cards the summary was written from. A different one\n" +
					"means the summary is about cards the board no longer holds."},
			{Name: "updated_at", Type: Millis()},
		},
		PK:  []string{"board_id"},
		FKs: []FK{boardFK("board_summary")},
	}
}

// inbox_sort is the log of where the inbox sorter put each card.
func inboxSort() Table {
	return Table{
		Name: "inbox_sort",
		Why: "Where the sorter put a card from the inbox, and why. A card that\n" +
			"turns up on a board nobody carried it to is the question this answers.\n" +
			"Both keys SET NULL: the decision is a fact about the sorter and\n" +
			"outlives the card and the board.",
		Columns: []Column{
			{Name: "id", Type: ID()},
			{Name: "card_id", Type: ID(), Null: true},
			{Name: "card_title", Type: Text(), Null: true,
				Why: "What the card was called, so the log still reads once it is gone."},
			{Name: "board_id", Type: ID(), Null: true,
				Why: "Where it went. NULL for a card left in the inbox."},
			{Name: "outcome", Type: Name(16)},
			{Name: "reason", Type: Text(), Null: true},
			{Name: "reason_code", Type: Name(64), Null: true},
			{Name: "reason_args", Type: Text(), Null: true},
			{Name: "created_at", Type: Millis()},
		},
		PK: []string{"id"},
		Checks: []Check{{
			Name: "inbox_sort_outcome", Column: "outcome",
			Values: []string{"moved", "kept", "failed"},
		}},
		FKs: []FK{
			{Name: "inbox_sort_card", Columns: []string{"card_id"},
				RefTable: "card_ref", RefCols: []string{"card_id"}, OnDelete: SetNull},
			{Name: "inbox_sort_board", Columns: []string{"board_id"},
				RefTable: "board_ref", RefCols: []string{"board_id"}, OnDelete: SetNull},
		},
	}
}
