document.addEventListener("DOMContentLoaded", () => {
    const notifToolbar = document.querySelector('[data-notif-page="1"]');
    const notifList = document.getElementById("notif-list");
    const unreadCountEl = document.getElementById("notif-unread-count");
    const badge = document.getElementById("notif-badge");
    const readAllForm = document.querySelector(".js-read-all-form");

    let state = {
        onlyUnread: false,
        limit: 20,
        cursor: 0,
        nextCursor: null,
        unreadCount: null,
    };

    function clampInt(v, min, max, fallback) {
        const n = Number.parseInt(String(v ?? ""), 10);
        if (Number.isNaN(n)) return fallback;
        return Math.max(min, Math.min(max, n));
    }

    function setBadgeCount(cnt) {
        if (!badge) return;
        if (cnt > 0) {
            badge.textContent = String(cnt);
            badge.classList.remove("is-hidden");
        } else {
            badge.textContent = "";
            badge.classList.add("is-hidden");
        }
    }

    function setUnreadCount(cnt) {
        state.unreadCount = Math.max(0, cnt | 0);
        if (unreadCountEl) unreadCountEl.textContent = String(state.unreadCount);
        setBadgeCount(state.unreadCount);

        if (readAllForm) {
            const btn = readAllForm.querySelector("button[type='submit']");
            if (btn) btn.disabled = state.unreadCount === 0;
        }
    }

    function formatTime(iso) {
        try {
            const d = new Date(iso);
            if (Number.isNaN(d.getTime())) return "";
            return new Intl.DateTimeFormat(undefined, {
                year: "numeric",
                month: "short",
                day: "2-digit",
                hour: "2-digit",
                minute: "2-digit",
            }).format(d);
        } catch (_) {
            return "";
        }
    }

    function gotoLabelForType(t) {
        if (t === "overspending") return "Go to budgets";
        if (t === "new_transaction") return "Go to transactions";
        return "Open";
    }

    function clearChildren(el) {
        while (el && el.firstChild) el.removeChild(el.firstChild);
    }

    function renderNotifications(items) {
        if (!notifList) return;

        clearChildren(notifList);

        if (!items || items.length === 0) {
            const empty = document.createElement("div");
            empty.className = "notif-empty";
            empty.textContent = "No notifications";
            notifList.appendChild(empty);
            return;
        }

        items.forEach((n) => {
            const card = document.createElement("article");
            card.className = `notif-card ${n.is_read ? "notif-read" : "notif-unread"}`;
            card.setAttribute("data-id", n.id);
            card.setAttribute("data-is-read", n.is_read ? "true" : "false");
            card.setAttribute("data-type", n.type || "");

            const head = document.createElement("header");
            head.className = "notif-head";

            const title = document.createElement("div");
            title.className = "notif-title";
            if (!n.is_read) {
                const dot = document.createElement("span");
                dot.className = "notif-dot";
                dot.setAttribute("aria-hidden", "true");
                title.appendChild(dot);
            }
            const titleText = document.createTextNode(n.title || "");
            title.appendChild(titleText);

            const time = document.createElement("div");
            time.className = "notif-time";
            time.textContent = formatTime(n.created_at);

            head.appendChild(title);
            head.appendChild(time);

            const body = document.createElement("p");
            body.className = "notif-body";
            body.textContent = n.body || "";

            const actions = document.createElement("footer");
            actions.className = "notif-actions";

            const goto = document.createElement("a");
            goto.className = "tx-btn tx-btn--ghost";
            goto.href = `/app/notifications/${encodeURIComponent(n.id)}/goto`;
            goto.textContent = gotoLabelForType(n.type);
            actions.appendChild(goto);

            if (!n.is_read) {
                const form = document.createElement("form");
                form.className = "js-mark-read-form";
                form.setAttribute("data-id", n.id);
                form.method = "post";
                form.action = `/app/notifications/${encodeURIComponent(n.id)}/read`;

                const btn = document.createElement("button");
                btn.className = "tx-btn tx-btn--primary";
                btn.type = "submit";
                btn.textContent = "Mark as read";

                form.appendChild(btn);
                actions.appendChild(form);
            } else {
                const lbl = document.createElement("span");
                lbl.className = "notif-read-label";
                lbl.textContent = "Read";
                actions.appendChild(lbl);
            }

            card.appendChild(head);
            card.appendChild(body);
            card.appendChild(actions);

            notifList.appendChild(card);
        });
    }

    async function apiFetch(url, options) {
        const res = await fetch(url, {
            credentials: "same-origin",
            ...options,
            headers: {
                Accept: "application/json",
                ...(options && options.headers ? options.headers : {}),
            },
        });
        return res;
    }

    async function loadNotifications() {
        if (!notifList) return;

        const url = `/api/notifications?onlyUnread=${state.onlyUnread ? "true" : "false"}&limit=${encodeURIComponent(
            state.limit
        )}&cursor=${encodeURIComponent(state.cursor)}`;

        const res = await apiFetch(url, { method: "GET" });
        if (!res.ok) return;
        const data = await res.json();
        const items = Array.isArray(data.notifications) ? data.notifications : [];

        const cnt = typeof data.unreadCount === "number" ? data.unreadCount :
            (typeof data.unread_count === "number" ? data.unread_count : 0);
        setUnreadCount(cnt);

        state.nextCursor = (typeof data.nextCursor === "number" ? data.nextCursor :
            (typeof data.next_cursor === "number" ? data.next_cursor : null));

        renderNotifications(items);
        bindDynamicHandlers();
        updatePager();
    }

    function setActiveFilterUI() {
        if (!notifToolbar) return;
        notifToolbar.querySelectorAll(".notif-filter").forEach((a) => {
            const isUnreadLink = (a.getAttribute("href") || "").includes("onlyUnread=true");
            const shouldBeActive = state.onlyUnread ? isUnreadLink : !isUnreadLink;
            if (shouldBeActive) a.classList.add("is-active");
            else a.classList.remove("is-active");
        });
    }

    function updateURL() {
        const params = new URLSearchParams();
        if (state.onlyUnread) params.set("onlyUnread", "true");
        params.set("limit", String(state.limit));
        if (state.cursor > 0) params.set("cursor", String(state.cursor));
        const qs = params.toString();
        const url = `/app/notifications${qs ? `?${qs}` : ""}`;
        try {
            window.history.replaceState({}, "", url);
        } catch (_) {
        }
    }

    async function markReadOptimistic(id, cardEl) {
        if (!id) return;
        const wasUnread = cardEl && cardEl.getAttribute("data-is-read") !== "true";

        if (cardEl) {
            cardEl.classList.remove("notif-unread");
            cardEl.classList.add("notif-read");
            cardEl.setAttribute("data-is-read", "true");

            const dot = cardEl.querySelector(".notif-dot");
            if (dot) dot.remove();

            const actions = cardEl.querySelector(".notif-actions");
            const form = actions ? actions.querySelector(".js-mark-read-form") : null;
            if (form) form.remove();
            if (actions) {
                const lbl = document.createElement("span");
                lbl.className = "notif-read-label";
                lbl.textContent = "Read";
                actions.appendChild(lbl);
            }

            if (state.onlyUnread) {
                cardEl.remove();
                if (notifList && notifList.children.length === 0) {
                    const empty = document.createElement("div");
                    empty.className = "notif-empty";
                    empty.textContent = "No notifications";
                    notifList.appendChild(empty);
                }
            }
        }

        if (wasUnread) setUnreadCount((state.unreadCount ?? 0) - 1);

        try {
            const res = await apiFetch(`/api/notifications/${encodeURIComponent(id)}`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ isRead: true }),
            });
            if (!res.ok) {
                await loadNotifications();
            }
        } catch (_) {
            await loadNotifications();
        }
    }

    async function markAllReadOptimistic() {
        const prevUnread = state.unreadCount ?? 0;
        if (prevUnread <= 0) return;

        setUnreadCount(0);
        if (notifList) {
            if (state.onlyUnread) {
                renderNotifications([]);
            } else {
                notifList.querySelectorAll(".notif-card").forEach((card) => {
                    card.classList.remove("notif-unread");
                    card.classList.add("notif-read");
                    card.setAttribute("data-is-read", "true");
                    const dot = card.querySelector(".notif-dot");
                    if (dot) dot.remove();
                    const form = card.querySelector(".js-mark-read-form");
                    if (form) form.remove();
                    const actions = card.querySelector(".notif-actions");
                    if (actions && !actions.querySelector(".notif-read-label")) {
                        const lbl = document.createElement("span");
                        lbl.className = "notif-read-label";
                        lbl.textContent = "Read";
                        actions.appendChild(lbl);
                    }
                });
            }
        }

        try {
            const res = await apiFetch(`/api/notifications/read-all`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
            });
            if (!res.ok) await loadNotifications();
        } catch (_) {
            await loadNotifications();
        }
    }

    function updatePager() {
        const pager = document.querySelector(".notif-pager");
        if (!pager) return;
        const nextLink = pager.querySelector(".js-notif-next");
        if (!nextLink) return;

        if (typeof state.nextCursor === "number") {
            nextLink.classList.remove("is-hidden");
            nextLink.setAttribute("data-next-cursor", String(state.nextCursor));
            const params = new URLSearchParams();
            if (state.onlyUnread) params.set("onlyUnread", "true");
            params.set("limit", String(state.limit));
            params.set("cursor", String(state.nextCursor));
            nextLink.href = `/app/notifications?${params.toString()}`;
        } else {
            nextLink.classList.add("is-hidden");
        }
    }

    function bindDynamicHandlers() {
        document.querySelectorAll(".js-mark-read-form").forEach((form) => {
            if (form.__bound) return;
            form.__bound = true;
            form.addEventListener("submit", (e) => {
                e.preventDefault();
                const id = form.getAttribute("data-id");
                const card = form.closest(".notif-card");
                markReadOptimistic(id, card);
            });
        });
    }

    async function refreshNotifBadge() {
        if (!badge) return;
        try {
            const res = await apiFetch("/api/notifications?onlyUnread=true&limit=1&cursor=0", {
                method: "GET",
            });
            if (!res.ok) return;
            const data = await res.json();
            const cnt = (typeof data.unreadCount === "number" ? data.unreadCount :
                (typeof data.unread_count === "number" ? data.unread_count : 0));
            setBadgeCount(cnt);
        } catch (_) {
        }
    }

    if (notifList && notifToolbar) {
        state.onlyUnread = (notifList.getAttribute("data-only-unread") === "true");
        state.limit = clampInt(notifList.getAttribute("data-limit"), 1, 100, 20);
        state.cursor = clampInt(notifList.getAttribute("data-cursor"), 0, 1_000_000, 0);

        notifToolbar.querySelectorAll(".notif-filter").forEach((a) => {
            a.addEventListener("click", (e) => {
                e.preventDefault();
                const href = a.getAttribute("href") || "";
                state.onlyUnread = href.includes("onlyUnread=true");
                state.cursor = 0;
                setActiveFilterUI();
                updateURL();
                loadNotifications();
            });
        });

        if (readAllForm && !readAllForm.__bound) {
            readAllForm.__bound = true;
            readAllForm.addEventListener("submit", (e) => {
                e.preventDefault();
                markAllReadOptimistic();
            });
        }

        const nextLink = document.querySelector(".js-notif-next");
        if (nextLink && !nextLink.__bound) {
            nextLink.__bound = true;
            nextLink.addEventListener("click", (e) => {
                const next = nextLink.getAttribute("data-next-cursor");
                const n = Number.parseInt(String(next || ""), 10);
                if (Number.isNaN(n)) return;
                e.preventDefault();
                state.cursor = Math.max(0, n);
                updateURL();
                loadNotifications();
            });
        }

        bindDynamicHandlers();
        loadNotifications();
        setActiveFilterUI();
        updateURL();
    } else {
        refreshNotifBadge();
    }
});
