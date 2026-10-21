/**
 * Opens a modal to browse and clear the AI generate memory of an automation.
 *
 * @param {string} automationId
 */
export function openAIMemoryModal(automationId) {
    const modal = aiMemoryModal(automationId);

    document.body.appendChild(modal);

    app.modals.open(modal);
}

function aiMemoryModal(automationId) {
    let modal;

    const baseURL = `/api/automations/${encodeURIComponent(automationId)}/ai-memory`;

    const data = store({
        isLoading: false,
        isBusy: false,
        sessions: [],
        expandedKey: "",
        messages: [],
        isLoadingMessages: false,
    });

    async function loadSessions() {
        data.isLoading = true;

        try {
            const result = await app.pb.send(baseURL, { requestKey: "aiMemorySessions" });
            data.sessions = result.items || [];
            if (data.expandedKey && !data.sessions.some((s) => s.key === data.expandedKey)) {
                data.expandedKey = "";
            }
            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                data.isLoading = false;
            }
        }
    }

    async function toggleSession(key) {
        if (data.expandedKey === key) {
            data.expandedKey = "";
            return;
        }

        data.expandedKey = key;
        data.messages = [];
        data.isLoadingMessages = true;

        try {
            const result = await app.pb.send(baseURL, {
                query: { key },
                requestKey: "aiMemoryMessages",
            });
            data.messages = result.messages || [];
            data.isLoadingMessages = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                data.isLoadingMessages = false;
            }
        }
    }

    function clearMemory(key = "") {
        app.modals.confirm(
            key
                ? `Clear the memory of session "${key}"?`
                : "Clear the memory of all sessions of this automation?",
            async () => {
                data.isBusy = true;

                try {
                    await app.pb.send(baseURL, {
                        method: "DELETE",
                        query: key ? { key } : {},
                        requestKey: null,
                    });
                    app.toasts.success("Memory cleared.");
                    await loadSessions();
                } catch (err) {
                    if (!err.isAbort) {
                        app.checkApiError(err);
                    }
                }

                data.isBusy = false;
            },
        );
    }

    modal = t.div(
        {
            pbEvent: "aiMemoryModal",
            className: "modal popup ai-memory-modal",
            onmount: () => loadSessions(),
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, "AI memory"),
        ),
        t.div(
            { className: "modal-content" },
            t.p(
                { className: "txt-hint txt-sm m-b-sm" },
                "Conversations remembered by the AI generate steps of this automation, grouped by session key.",
            ),
            t.div(
                { className: "list ai-memory-list" },
                () =>
                    data.isLoading && !data.sessions.length
                        ? t.div({ className: "list-item" }, t.div({ className: "skeleton-loader" }))
                        : null,
                () =>
                    !data.isLoading && !data.sessions.length
                        ? t.div({ className: "list-item" }, t.div({ className: "content txt-hint" }, "No memory yet."))
                        : null,
                () =>
                    data.sessions.map((session) =>
                        t.div(
                            { rid: session.key, className: "ai-memory-session" },
                            t.div(
                                { className: "list-item" },
                                t.button(
                                    {
                                        type: "button",
                                        className: "content ai-memory-session-toggle",
                                        onclick: () => toggleSession(session.key),
                                    },
                                    t.i({
                                        className: () =>
                                            data.expandedKey === session.key
                                                ? "ri-arrow-down-s-line"
                                                : "ri-arrow-right-s-line",
                                        ariaHidden: true,
                                    }),
                                    t.code({ className: "txt-ellipsis", title: session.key }, session.key),
                                    t.span({ className: "txt-hint txt-sm" }, `${session.messages} messages`),
                                    t.span(
                                        { className: "txt-hint txt-sm m-l-auto" },
                                        session.updated ? app.utils.toLocalDatetime(session.updated) : "",
                                    ),
                                ),
                                t.nav(
                                    { className: "actions" },
                                    t.button(
                                        {
                                            type: "button",
                                            className: "btn sm circle danger transparent",
                                            ariaLabel: app.attrs.tooltip("Clear session"),
                                            disabled: () => data.isBusy,
                                            onclick: () => clearMemory(session.key),
                                        },
                                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                                    ),
                                ),
                            ),
                            () => {
                                if (data.expandedKey !== session.key) {
                                    return null;
                                }

                                if (data.isLoadingMessages) {
                                    return t.div(
                                        { className: "ai-memory-messages" },
                                        t.div({ className: "skeleton-loader" }),
                                    );
                                }

                                return t.div(
                                    { className: "ai-memory-messages" },
                                    data.messages.map((message) =>
                                        t.div(
                                            { className: `ai-memory-message ${message.role}` },
                                            t.div(
                                                { className: "ai-memory-role txt-hint txt-sm" },
                                                message.role === "user" ? "User" : "AI",
                                            ),
                                            t.div({ className: "ai-memory-content" }, message.content),
                                        )
                                    ),
                                );
                            },
                        )
                    ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent danger m-r-auto",
                    disabled: () => data.isBusy || !data.sessions.length,
                    onclick: () => clearMemory(),
                },
                t.span({ className: "txt" }, "Clear all"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn secondary",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
        ),
    );

    return modal;
}
