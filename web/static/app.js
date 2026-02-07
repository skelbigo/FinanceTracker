// Placeholder for Stage 1.
// Later: handle flash clearing, HTMX events, etc.


document.addEventListener("DOMContentLoaded", () => {
    async function postJSON(url) {
        const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" } });
        return res;
    }

    document.querySelectorAll(".js-mark-read").forEach((btn) => {
        btn.addEventListener("click", async () => {
            const id = btn.getAttribute("data-id");
            if (!id) return;
            const res = await postJSON(`/api/notifications/${encodeURIComponent(id)}/read`);
            if (res.ok) window.location.reload();
        });
    });

    const allBtn = document.querySelector(".js-mark-all-read");
    if (allBtn) {
        allBtn.addEventListener("click", async () => {
            const res = await postJSON(`/api/notifications/read-all`);
            if (res.ok) window.location.reload();
        });
    }
});
