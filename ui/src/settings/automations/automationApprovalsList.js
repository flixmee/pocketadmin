export function automationApprovalsList(propsArg = {}) {
    const props = store({
        reset: null,
    });
    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isResolving: {},
        approvals: [],
    });

    async function loadApprovals() {
        data.isLoading = true;

        try {
            data.approvals = await app.pb.send("/api/automations/approvals?status=pending", {
                requestKey: "automationApprovalsList.load",
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoading = false;
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
            `${label} approval "${approval.id}"?`,
            () => resolveApproval(approval, decision),
            null,
            { yesButton: label, noButton: "Cancel" },
        );
    }

    return t.div(
        {
            pbEvent: "automationApprovalsList",
            className: "al-card-list",
            onmount: () => {
                loadApprovals();
                watchers.push(watch(() => props.reset, () => loadApprovals()));
            },
            onunmount: () => watchers.forEach((w) => w?.unwatch()),
        },
        // Loading skeleton
        t.div(
            {
                hidden: () => !data.isLoading || data.approvals.length,
                className: "al-card-row",
            },
            t.div({ className: "skeleton-loader" }),
        ),
        // Empty state
        t.div(
            {
                hidden: () => data.isLoading || data.approvals.length,
                className: "al-card-row al-empty-state",
            },
            t.div(
                { className: "al-empty-icon-wrap al-empty-icon-success" },
                t.i({ className: "ri-check-line", ariaHidden: true }),
            ),
            t.div({ className: "al-empty-title" }, "All caught up"),
            t.div(
                { className: "al-empty-hint" },
                "Pending approvals will appear here when an automation requires manual review.",
            ),
        ),
        // Approval rows
        () =>
            data.approvals.map((approval) => {
                return t.div(
                    { className: () => `al-card-row ${data.isLoading ? "al-faded" : ""}` },
                    // Icon block
                    t.div(
                        { className: "al-icon-block al-icon-amber" },
                        t.i({ className: "ri-shield-check-line", ariaHidden: true }),
                    ),
                    // Content
                    t.div(
                        { className: "al-row-content" },
                        t.div(
                            { className: "al-row-top" },
                            t.span({ className: "al-row-name txt-code" }, () => approval.id),
                            t.span({ className: "al-badge al-badge-amber" }, "Pending"),
                            () =>
                                approval.role ? t.span({ className: "al-badge al-badge-muted" }, approval.role) : null,
                            () =>
                                approval.assignee
                                    ? t.span({ className: "al-badge al-badge-muted" }, approval.assignee)
                                    : null,
                        ),
                        t.div(
                            { className: "al-row-meta" },
                            () => `Run ${approval.runRef || "N/A"}`,
                            t.span({ className: "al-meta-dot" }, "·"),
                            () => `Step ${Number(approval.stepIndex || 0) + 1}`,
                        ),
                    ),
                    // Actions
                    t.div(
                        { className: "al-row-actions" },
                        t.button(
                            {
                                type: "button",
                                className: () =>
                                    `al-action-btn al-action-success ${data.isResolving[approval.id] ? "loading" : ""}`,
                                disabled: () => data.isLoading || !!data.isResolving[approval.id],
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
                                disabled: () => data.isLoading || !!data.isResolving[approval.id],
                                ariaLabel: app.attrs.tooltip("Reject"),
                                onclick: () => confirmDecision(approval, "rejected"),
                            },
                            t.i({ className: "ri-close-line", ariaHidden: true }),
                        ),
                    ),
                );
            }),
    );
}
