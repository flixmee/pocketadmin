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
            className: "list automation-approvals-list",
            onmount: () => {
                loadApprovals();
                watchers.push(watch(() => props.reset, () => loadApprovals()));
            },
            onunmount: () => watchers.forEach((w) => w?.unwatch()),
        },
        t.div(
            { className: "list-content" },
            t.div(
                {
                    hidden: () => !data.isLoading || data.approvals.length,
                    className: "list-item",
                },
                t.div({ className: "skeleton-loader" }),
            ),
            t.div(
                {
                    hidden: () => data.isLoading || data.approvals.length,
                    className: "list-item",
                },
                t.div({ className: "content block txt-hint" }, "No pending approvals."),
            ),
            () =>
                data.approvals.map((approval) => {
                    return t.div(
                        { className: () => `list-item ${data.isLoading ? "faded" : ""}` },
                        t.i({ className: "ri-shield-check-line txt-warning", ariaHidden: true }),
                        t.div(
                            { className: "content block" },
                            t.div(
                                { className: "flex flex-wrap gap-5" },
                                t.span({ className: "txt-bold txt-code" }, () => approval.id),
                                t.span({ className: "label warning" }, "Pending"),
                                () => approval.role ? t.span({ className: "label" }, approval.role) : null,
                                () => approval.assignee ? t.span({ className: "label" }, approval.assignee) : null,
                            ),
                            t.div(
                                { className: "txt-sm txt-hint m-t-5" },
                                () => `Run ${approval.runRef || "N/A"} • Step ${Number(approval.stepIndex || 0) + 1}`,
                            ),
                        ),
                        t.nav(
                            { className: "actions" },
                            t.button(
                                {
                                    type: "button",
                                    className: () =>
                                        `btn sm circle success transparent ${
                                            data.isResolving[approval.id] ? "loading" : ""
                                        }`,
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
                                        `btn sm circle danger transparent ${
                                            data.isResolving[approval.id] ? "loading" : ""
                                        }`,
                                    disabled: () => data.isLoading || !!data.isResolving[approval.id],
                                    ariaLabel: app.attrs.tooltip("Reject"),
                                    onclick: () => confirmDecision(approval, "rejected"),
                                },
                                t.i({ className: "ri-close-line", ariaHidden: true }),
                            ),
                        ),
                    );
                }),
        ),
    );
}
