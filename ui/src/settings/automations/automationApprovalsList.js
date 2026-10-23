const historyLimit = 100;

const statusBadges = {
    pending: { label: "Pending", className: "al-badge-amber" },
    approved: { label: "Approved", className: "al-badge-green" },
    rejected: { label: "Rejected", className: "al-badge-red" },
};

function approvalSearchText(approval) {
    return [
        approval.automationName,
        approval.id,
        approval.runRef,
        approval.assignee,
        approval.role,
        approval.comment,
    ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
}

export function automationApprovalsList(propsArg = {}) {
    const props = store({
        reset: null,
        // called with the number of pending approvals after each load
        oncount: null,
    });
    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        view: "pending", // pending | history
        keyword: "",
        isLoading: false,
        isLoadingHistory: false,
        isBulkResolving: false,
        isResolving: {},
        approvals: [],
        history: [],
        selected: {},
        get items() {
            return data.view === "history" ? data.history : data.approvals;
        },
        get visibleItems() {
            const keyword = data.keyword.trim().toLowerCase();
            if (!keyword) {
                return data.items;
            }
            return data.items.filter((approval) => approvalSearchText(approval).includes(keyword));
        },
        get selectedIds() {
            return data.approvals.filter((approval) => data.selected[approval.id]).map((approval) => approval.id);
        },
        get allVisibleSelected() {
            const visible = data.visibleItems;
            return visible.length > 0 && visible.every((approval) => data.selected[approval.id]);
        },
    });

    let realtimeUnsubscribe = null;
    let realtimeRefreshTimer = null;
    let pendingRealtimeRefresh = false;
    let isMounted = false;

    async function loadApprovals() {
        if (data.isLoading) {
            pendingRealtimeRefresh = true;
            return;
        }

        data.isLoading = true;

        try {
            const approvals = await app.pb.send("/api/automations/approvals?status=pending", {
                requestKey: "automationApprovalsList.load",
            });
            data.approvals = approvals;

            // drop the selected approvals that are no longer pending
            const pendingIds = new Set(approvals.map((approval) => approval.id));
            for (const id of Object.keys(data.selected)) {
                if (!pendingIds.has(id)) {
                    delete data.selected[id];
                }
            }

            props.oncount?.(approvals.length);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoading = false;

        if (data.view === "history") {
            loadHistory();
        }

        if (pendingRealtimeRefresh && isMounted) {
            pendingRealtimeRefresh = false;
            queueRealtimeRefresh();
        }
    }

    async function loadHistory() {
        data.isLoadingHistory = true;

        try {
            data.history = await app.pb.send(`/api/automations/approvals?status=resolved&limit=${historyLimit}`, {
                requestKey: "automationApprovalsList.history",
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoadingHistory = false;
    }

    function setView(view) {
        data.view = view;
        if (view === "history") {
            loadHistory();
        }
    }

    function queueRealtimeRefresh() {
        if (!isMounted) {
            return;
        }

        clearTimeout(realtimeRefreshTimer);
        realtimeRefreshTimer = setTimeout(() => {
            loadApprovals();
        }, 150);
    }

    async function subscribeToRealtime() {
        try {
            const unsubscribe = await app.pb.collection("_approvals").subscribe("*", queueRealtimeRefresh);
            if (!isMounted) {
                unsubscribe().catch((err) => {
                    console.warn("Failed to unsubscribe from automation approvals realtime updates:", err);
                });
                return;
            }

            realtimeUnsubscribe = unsubscribe;
        } catch (err) {
            console.warn("Failed to subscribe to automation approvals realtime updates:", err);
        }
    }

    function unsubscribeFromRealtime() {
        isMounted = false;
        pendingRealtimeRefresh = false;
        clearTimeout(realtimeRefreshTimer);
        realtimeRefreshTimer = null;

        if (typeof realtimeUnsubscribe === "function") {
            realtimeUnsubscribe().catch((err) => {
                console.warn("Failed to unsubscribe from automation approvals realtime updates:", err);
            });
        }

        realtimeUnsubscribe = null;
    }

    async function resolveApproval(approval, decision) {
        if (!approval?.id || data.isResolving[approval.id]) {
            return;
        }

        data.isResolving[approval.id] = true;

        try {
            await app.pb.send(`/api/automations/approvals/${approval.id}/decision`, {
                method: "POST",
                body: { decision },
            });

            app.toasts.success(decision === "approved" ? "Approval accepted." : "Approval rejected.");
            delete data.selected[approval.id];
            await loadApprovals();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isResolving[approval.id];
    }

    function confirmDecision(approval, decision) {
        const label = decision === "approved" ? "Approve" : "Reject";
        app.modals.confirm(
            `${label} the approval of "${approval.automationName || approval.id}"?`,
            () => resolveApproval(approval, decision),
            null,
            { yesButton: label, noButton: "Cancel" },
        );
    }

    async function resolveSelected(decision) {
        const ids = data.selectedIds;
        if (!ids.length || data.isBulkResolving) {
            return;
        }

        data.isBulkResolving = true;

        try {
            const result = await app.pb.send("/api/automations/approvals/decisions", {
                method: "POST",
                body: { ids, decision },
                requestKey: null,
            });

            const resolved = result?.resolved?.length || 0;
            const failed = result?.failed || [];

            if (resolved) {
                app.toasts.success(
                    `${decision === "approved" ? "Approved" : "Rejected"} ${resolved} approval${
                        resolved === 1 ? "" : "s"
                    }.`,
                );
            }
            if (failed.length) {
                app.toasts.error(
                    `${failed.length} approval${failed.length === 1 ? "" : "s"} couldn't be resolved: ${
                        failed[0].error
                    }`,
                );
            }

            data.selected = {};
            await loadApprovals();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isBulkResolving = false;
    }

    function confirmBulkDecision(decision) {
        const count = data.selectedIds.length;
        const label = decision === "approved" ? "Approve" : "Reject";
        app.modals.confirm(
            `${label} ${count} selected approval${
                count === 1 ? "" : "s"
            }? The paused automations will continue accordingly.`,
            () => resolveSelected(decision),
            null,
            { yesButton: `${label} ${count}`, noButton: "Cancel" },
        );
    }

    function toggleAllVisible(checked) {
        const selected = { ...data.selected };
        for (const approval of data.visibleItems) {
            if (checked) {
                selected[approval.id] = true;
            } else {
                delete selected[approval.id];
            }
        }
        data.selected = selected;
    }

    function renderRow(approval) {
        const isPending = approval.status === "pending";
        const badge = statusBadges[approval.status] || { label: approval.status, className: "al-badge-muted" };

        return t.div(
            {
                rid: approval.id,
                className: () =>
                    `al-card-row al-approval-row ${data.isLoading ? "al-faded" : ""} ${
                        data.selected[approval.id] ? "al-selected" : ""
                    }`,
            },
            isPending
                ? t.label(
                    { className: "al-row-check", ariaLabel: "Select approval" },
                    t.input({
                        type: "checkbox",
                        checked: () => !!data.selected[approval.id],
                        disabled: () => data.isBulkResolving,
                        onchange: (e) => {
                            if (e.target.checked) {
                                data.selected[approval.id] = true;
                            } else {
                                delete data.selected[approval.id];
                            }
                        },
                    }),
                )
                : null,
            t.div(
                { className: `al-icon-block ${isPending ? "al-icon-amber" : ""}` },
                t.i({ className: isPending ? "ri-shield-check-line" : "ri-history-line", ariaHidden: true }),
            ),
            t.div(
                { className: "al-row-content" },
                t.div(
                    { className: "al-row-top" },
                    approval.automationRef
                        ? t.a(
                            {
                                className: "al-row-name",
                                href: `#/automations/${approval.automationRef}`,
                                title: approval.automationName || approval.automationRef,
                            },
                            approval.automationName || approval.automationRef,
                        )
                        : t.span({ className: "al-row-name" }, "Unknown automation"),
                    t.span({ className: `al-badge ${badge.className}` }, badge.label),
                    approval.role ? t.span({ className: "al-badge al-badge-muted" }, approval.role) : null,
                    approval.assignee ? t.span({ className: "al-badge al-badge-muted" }, approval.assignee) : null,
                ),
                t.div(
                    { className: "al-row-meta" },
                    t.span(null, `Step ${Number(approval.stepIndex || 0) + 1}`),
                    t.span({ className: "al-meta-dot" }, "·"),
                    t.span({ className: "txt-code", title: "Run ID" }, approval.runRef || "N/A"),
                    t.span({ className: "al-meta-dot" }, "·"),
                    t.span(
                        { title: "Requested" },
                        isPending
                            ? `Requested ${app.utils.toLocalDatetime(approval.created)}`
                            : `Resolved ${app.utils.toLocalDatetime(approval.resolved || approval.updated)}`,
                    ),
                    approval.comment
                        ? t.span(
                            { className: "al-approval-comment", title: approval.comment },
                            t.span({ className: "al-meta-dot" }, "·"),
                            `“${approval.comment}”`,
                        )
                        : null,
                ),
            ),
            isPending
                ? t.div(
                    { className: "al-row-actions" },
                    t.button(
                        {
                            type: "button",
                            className: () =>
                                `al-action-btn al-action-success ${data.isResolving[approval.id] ? "loading" : ""}`,
                            disabled: () => data.isLoading || data.isBulkResolving || !!data.isResolving[approval.id],
                            ariaLabel: app.attrs.tooltip("Approve"),
                            onclick: () => confirmDecision(approval, "approved"),
                        },
                        t.i({ className: "ri-check-line", ariaHidden: true }),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: () =>
                                `al-action-btn al-action-danger ${data.isResolving[approval.id] ? "loading" : ""}`,
                            disabled: () => data.isLoading || data.isBulkResolving || !!data.isResolving[approval.id],
                            ariaLabel: app.attrs.tooltip("Reject"),
                            onclick: () => confirmDecision(approval, "rejected"),
                        },
                        t.i({ className: "ri-close-line", ariaHidden: true }),
                    ),
                )
                : null,
        );
    }

    return t.div(
        {
            pbEvent: "automationApprovalsList",
            className: "al-approvals",
            onmount: () => {
                isMounted = true;
                loadApprovals();
                subscribeToRealtime();
                watchers.push(watch(() => props.reset, () => loadApprovals()));
            },
            onunmount: () => {
                unsubscribeFromRealtime();
                watchers.forEach((w) => w?.unwatch());
            },
        },
        // Toolbar
        t.div(
            { className: "al-list-toolbar al-approvals-toolbar" },
            t.div(
                { className: "al-segmented", role: "tablist" },
                t.button(
                    {
                        type: "button",
                        role: "tab",
                        className: () => `al-segment ${data.view === "pending" ? "active" : ""}`,
                        ariaSelected: () => data.view === "pending",
                        onclick: () => setView("pending"),
                    },
                    "Pending",
                    t.span({ className: "al-segment-count" }, () => data.approvals.length),
                ),
                t.button(
                    {
                        type: "button",
                        role: "tab",
                        className: () => `al-segment ${data.view === "history" ? "active" : ""}`,
                        ariaSelected: () => data.view === "history",
                        onclick: () => setView("history"),
                    },
                    "History",
                ),
            ),
            t.div(
                { className: "al-search-field" },
                t.i({ className: "ri-search-line", ariaHidden: true }),
                t.input({
                    type: "search",
                    placeholder: "Search by automation, assignee, role or run",
                    value: () => data.keyword,
                    oninput: (e) => (data.keyword = e.target.value),
                }),
            ),
        ),
        // Bulk actions bar (pending only)
        () => {
            if (data.view !== "pending" || !data.approvals.length) {
                return null;
            }

            const selectedCount = data.selectedIds.length;

            return t.div(
                { className: () => `al-bulk-bar ${selectedCount ? "active" : ""}` },
                t.label(
                    { className: "al-bulk-select-all" },
                    t.input({
                        type: "checkbox",
                        checked: () => data.allVisibleSelected,
                        disabled: () => !data.visibleItems.length || data.isBulkResolving,
                        onchange: (e) => toggleAllVisible(e.target.checked),
                    }),
                    t.span(
                        null,
                        selectedCount
                            ? `${selectedCount} selected`
                            : `Select all (${data.visibleItems.length})`,
                    ),
                ),
                selectedCount
                    ? t.div(
                        { className: "al-bulk-actions" },
                        t.button(
                            {
                                type: "button",
                                className: () => `btn sm success ${data.isBulkResolving ? "loading" : ""}`,
                                disabled: () => data.isBulkResolving,
                                onclick: () => confirmBulkDecision("approved"),
                            },
                            t.i({ className: "ri-check-double-line", ariaHidden: true }),
                            t.span({ className: "txt" }, `Approve ${selectedCount}`),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: () => `btn sm danger ${data.isBulkResolving ? "loading" : ""}`,
                                disabled: () => data.isBulkResolving,
                                onclick: () => confirmBulkDecision("rejected"),
                            },
                            t.i({ className: "ri-close-line", ariaHidden: true }),
                            t.span({ className: "txt" }, `Reject ${selectedCount}`),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm transparent secondary",
                                disabled: () => data.isBulkResolving,
                                onclick: () => (data.selected = {}),
                            },
                            t.span({ className: "txt" }, "Clear"),
                        ),
                    )
                    : t.span({ className: "txt-hint txt-sm" }, "Select approvals to approve or reject them together."),
            );
        },
        t.div(
            { className: "al-card-list" },
            // Loading skeleton
            t.div(
                {
                    hidden: () =>
                        data.view === "history"
                            ? !data.isLoadingHistory || data.history.length
                            : !data.isLoading || data.approvals.length,
                    className: "al-card-row",
                },
                t.div({ className: "skeleton-loader" }),
            ),
            // Empty states
            () => {
                const loading = data.view === "history" ? data.isLoadingHistory : data.isLoading;
                if (loading || data.visibleItems.length) {
                    return null;
                }

                if (data.items.length) {
                    return t.div(
                        { className: "al-card-row al-empty-state" },
                        t.div(
                            { className: "al-empty-icon-wrap" },
                            t.i({ className: "ri-search-eye-line", ariaHidden: true }),
                        ),
                        t.div({ className: "al-empty-title" }, "No matching approvals"),
                        t.div({ className: "al-empty-hint" }, "Try a different search."),
                    );
                }

                if (data.view === "history") {
                    return t.div(
                        { className: "al-card-row al-empty-state" },
                        t.div(
                            { className: "al-empty-icon-wrap" },
                            t.i({ className: "ri-history-line", ariaHidden: true }),
                        ),
                        t.div({ className: "al-empty-title" }, "No resolved approvals yet"),
                        t.div(
                            { className: "al-empty-hint" },
                            `The last ${historyLimit} approved or rejected requests appear here.`,
                        ),
                    );
                }

                return t.div(
                    { className: "al-card-row al-empty-state" },
                    t.div(
                        { className: "al-empty-icon-wrap al-empty-icon-success" },
                        t.i({ className: "ri-check-line", ariaHidden: true }),
                    ),
                    t.div({ className: "al-empty-title" }, "All caught up"),
                    t.div(
                        { className: "al-empty-hint" },
                        "Pending approvals will appear here when an automation requires manual review.",
                    ),
                );
            },
            () => data.visibleItems.map((approval) => renderRow(approval)),
        ),
    );
}
