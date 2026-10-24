const conversationsPageSize = 50;
const messagesPageSize = 50;
const messagesMaxReload = 200;
const phoneThreadPrefix = "phone:";

const outboxStatuses = {
    pending: { icon: "ri-time-line", label: "Queued" },
    sending: { icon: "ri-loader-4-line", label: "Sending" },
    sent: { icon: "ri-check-line", label: "Sent" },
    failed: { icon: "ri-error-warning-line", label: "Failed" },
};

/**
 * Zalo inbox: the conversations list, the opened conversation and the composer.
 *
 * @param  {Object}   props
 * @param  {Function} props.channel Returns the Zalo channel state (see /api/conversations/channels).
 * @return {Element}
 */
export function zaloConversations(props = {}) {
    const data = store({
        conversations: [],
        isLoadingList: false,
        listHasMore: false,
        search: "",
        selectedId: "",
        // the opened conversation (kept even when it isn't in the loaded list page)
        selected: null,
        messages: [],
        messagesHasMore: false,
        isLoadingMessages: false,
        isLoadingOlder: false,
        draft: "",
        replyTo: null,
        isSending: false,
    });

    let messagesEl = null;
    let composerEl = null;
    let searchTimer = 0;
    let listRefreshTimer = 0;
    let messagesRefreshTimer = 0;
    let unsubscribers = [];
    let isDestroyed = false;

    async function loadConversations(reset = true) {
        data.isLoadingList = true;

        try {
            const params = new URLSearchParams({
                search: data.search.trim(),
                limit: conversationsPageSize,
                offset: reset ? 0 : data.conversations.length,
            });
            const items = await app.pb.send(`/api/conversations/zalo?${params}`, {
                requestKey: "zaloConversations.list",
            });

            data.conversations = reset ? items : data.conversations.concat(items);
            data.listHasMore = items.length === conversationsPageSize;

            // keep the opened conversation details up to date
            const selected = data.conversations.find((item) => item.id === data.selectedId);
            if (selected) {
                data.selected = selected;
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoadingList = false;
    }

    function selectConversation(conversation) {
        if (!conversation) {
            return;
        }

        if (data.selectedId !== conversation.id) {
            data.draft = "";
            data.replyTo = null;
            data.messages = [];
            data.messagesHasMore = false;
        }

        data.selectedId = conversation.id;
        data.selected = conversation;
        loadMessages();
        focusComposer();
    }

    function closeConversation() {
        data.selectedId = "";
        data.selected = null;
        data.messages = [];
        data.replyTo = null;
    }

    async function loadMessages(silent = false) {
        const conversationId = data.selectedId;
        if (!conversationId) {
            return;
        }

        if (!silent) {
            data.isLoadingMessages = true;
        }

        const stickToBottom = !silent || isNearBottom();

        try {
            const limit = Math.min(Math.max(messagesPageSize, data.messages.length), messagesMaxReload);
            const result = await app.pb.send(
                `/api/conversations/zalo/${encodeURIComponent(conversationId)}/messages?limit=${limit}`,
                { requestKey: "zaloConversations.messages" },
            );

            // another conversation was opened in the meantime
            if (conversationId !== data.selectedId) {
                return;
            }

            data.messages = result.items || [];
            data.messagesHasMore = !!result.hasMore;

            if (stickToBottom) {
                scrollToBottom();
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoadingMessages = false;
    }

    async function loadOlderMessages() {
        const conversationId = data.selectedId;
        const oldest = data.messages[0];
        if (!conversationId || !oldest || data.isLoadingOlder) {
            return;
        }

        data.isLoadingOlder = true;

        try {
            const params = new URLSearchParams({ limit: messagesPageSize, before: oldest.created });
            const result = await app.pb.send(
                `/api/conversations/zalo/${encodeURIComponent(conversationId)}/messages?${params}`,
                { requestKey: "zaloConversations.olderMessages" },
            );

            if (conversationId === data.selectedId) {
                // keep the visible messages in place
                const previousHeight = messagesEl?.scrollHeight || 0;
                data.messages = (result.items || []).concat(data.messages);
                data.messagesHasMore = !!result.hasMore;
                requestAnimationFrame(() => {
                    if (messagesEl) {
                        messagesEl.scrollTop += messagesEl.scrollHeight - previousHeight;
                    }
                });
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoadingOlder = false;
    }

    async function send() {
        const text = data.draft.trim();
        if (!text || !data.selectedId || data.isSending) {
            return;
        }

        data.isSending = true;

        try {
            const message = await app.pb.send("/api/conversations/zalo/messages", {
                method: "POST",
                body: {
                    threadId: data.selectedId,
                    threadType: data.selected?.threadType || "user",
                    text: text,
                    replyTo: data.replyTo?.id || "",
                },
                requestKey: null,
            });

            data.draft = "";
            data.replyTo = null;
            if (!data.messages.some((item) => item.id === message.id)) {
                data.messages = data.messages.concat(message);
            }
            scrollToBottom();
            queueListRefresh();
        } catch (err) {
            if (!err?.isAbort) {
                app.toasts.error(apiErrorMessage(err, "Failed to send the message."));
            }
        }

        data.isSending = false;
        focusComposer();
    }

    function openNewMessage() {
        openZaloNewMessageModal({
            onsent: (message) => {
                const conversation = {
                    id: message.threadId,
                    threadType: message.threadType || "user",
                    name: message.threadId.startsWith(phoneThreadPrefix)
                        ? message.threadId.substring(phoneThreadPrefix.length)
                        : message.threadId,
                    lastMessage: message,
                };
                selectConversation(conversation);
                loadConversations();
            },
        });
    }

    // realtime
    // ---------------------------------------------------------------

    function queueListRefresh() {
        clearTimeout(listRefreshTimer);
        listRefreshTimer = setTimeout(() => loadConversations(), 400);
    }

    function queueMessagesRefresh() {
        clearTimeout(messagesRefreshTimer);
        messagesRefreshTimer = setTimeout(() => loadMessages(true), 250);
    }

    function handleRealtimeEvent(collection, e) {
        const record = e?.record || {};

        let threadId = record.thread_id || "";
        if (collection === "zalo_outbox") {
            threadId = record.resolved_thread_id || record.thread_id || phoneThreadPrefix + record.phone;

            // the bridge resolved the phone number of the opened conversation
            const selectedPhone = data.selectedId.startsWith(phoneThreadPrefix)
                ? data.selectedId.substring(phoneThreadPrefix.length)
                : "";
            if (selectedPhone && record.resolved_thread_id && record.phone === selectedPhone) {
                selectConversation({
                    ...data.selected,
                    id: record.resolved_thread_id,
                    threadType: record.thread_type || "user",
                });
            }
        }

        queueListRefresh();
        if (threadId && threadId === data.selectedId) {
            queueMessagesRefresh();
        }
    }

    async function subscribe() {
        for (const collection of ["zalo_messages", "zalo_outbox"]) {
            try {
                const unsubscribe = await app.pb.collection(collection).subscribe("*", (e) => {
                    handleRealtimeEvent(collection, e);
                });
                if (isDestroyed) {
                    unsubscribe().catch(() => {});
                } else {
                    unsubscribers.push(unsubscribe);
                }
            } catch (err) {
                console.warn(`failed to subscribe to ${collection}:`, err);
            }
        }
    }

    // helpers
    // ---------------------------------------------------------------

    function isNearBottom() {
        if (!messagesEl) {
            return true;
        }
        return messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight < 120;
    }

    function scrollToBottom() {
        requestAnimationFrame(() => {
            if (messagesEl) {
                messagesEl.scrollTop = messagesEl.scrollHeight;
            }
        });
    }

    function focusComposer() {
        requestAnimationFrame(() => composerEl?.focus());
    }

    function startReply(message) {
        data.replyTo = message;
        focusComposer();
    }

    // render
    // ---------------------------------------------------------------

    return t.div(
        {
            pbEvent: "zaloConversations",
            className: () => `conversations-layout zalo-inbox ${data.selectedId ? "has-selection" : ""}`,
            onmount: () => {
                loadConversations();
                subscribe();
            },
            onunmount: () => {
                isDestroyed = true;
                clearTimeout(searchTimer);
                clearTimeout(listRefreshTimer);
                clearTimeout(messagesRefreshTimer);
                unsubscribers.forEach((unsubscribe) => unsubscribe().catch(() => {}));
                unsubscribers = [];
            },
        },
        // conversations list
        t.aside(
            { className: "conversations-sidebar" },
            t.div(
                { className: "conversations-sidebar-header" },
                t.div(
                    { className: "conversations-search" },
                    t.i({ className: "ri-search-line", ariaHidden: true }),
                    t.input({
                        type: "search",
                        placeholder: "Search name, phone or message...",
                        ariaLabel: "Search conversations",
                        value: () => data.search,
                        oninput: (e) => {
                            data.search = e.target.value;
                            clearTimeout(searchTimer);
                            searchTimer = setTimeout(() => loadConversations(), 300);
                        },
                    }),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn sm circle",
                        ariaLabel: app.attrs.tooltip("New message"),
                        onclick: openNewMessage,
                    },
                    t.i({ className: "ri-edit-box-line", ariaHidden: true }),
                ),
            ),
            t.div(
                { className: "conversations-list" },
                () => {
                    if (!data.conversations.length) {
                        if (data.isLoadingList) {
                            return t.div({ className: "conversations-list-empty" }, t.span({ className: "loader" }));
                        }
                        return t.div(
                            { className: "conversations-list-empty" },
                            data.search.trim() ? "No conversations match your search." : "No conversations yet.",
                        );
                    }

                    return data.conversations.map((conversation) => conversationItem(conversation));
                },
                () =>
                    data.listHasMore
                        ? t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent block conversations-load-more",
                                disabled: () => data.isLoadingList,
                                onclick: () => loadConversations(false),
                            },
                            t.span({ className: "txt" }, "Load more"),
                        )
                        : null,
            ),
        ),
        // opened conversation
        t.section(
            { className: "conversation-thread" },
            () => {
                if (!data.selectedId) {
                    return t.div(
                        { className: "conversation-empty" },
                        t.i({ className: "ri-chat-3-line", ariaHidden: true }),
                        t.div({ className: "txt-bold" }, "Select a conversation"),
                        t.div({ className: "txt-hint" }, "Or start a new one with a Zalo user id or a phone number."),
                        t.button(
                            { type: "button", className: "btn sm secondary", onclick: openNewMessage },
                            t.i({ className: "ri-edit-box-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "New message"),
                        ),
                    );
                }

                return [
                    threadHeader(),
                    t.div(
                        {
                            className: "conversation-messages",
                            onmount: (el) => (messagesEl = el),
                            onunmount: () => (messagesEl = null),
                        },
                        () =>
                            data.messagesHasMore
                                ? t.button(
                                    {
                                        type: "button",
                                        className: "btn xs secondary transparent conversation-load-older",
                                        disabled: () => data.isLoadingOlder,
                                        onclick: loadOlderMessages,
                                    },
                                    t.span({ className: "txt" }, "Load older messages"),
                                )
                                : null,
                        () => {
                            if (!data.messages.length) {
                                return t.div(
                                    { className: "conversation-messages-empty" },
                                    data.isLoadingMessages ? t.span({ className: "loader" }) : "No messages yet.",
                                );
                            }
                            return renderMessages(data.messages, data.selected);
                        },
                    ),
                    composer(),
                ];
            },
        ),
    );

    function conversationItem(conversation) {
        const last = conversation.lastMessage;
        const status = last?.source === "outbox" && last.status !== "sent" ? outboxStatuses[last.status] : null;

        return t.button(
            {
                type: "button",
                className: () => `conversation-item ${data.selectedId === conversation.id ? "active" : ""}`,
                onclick: () => selectConversation(conversation),
            },
            avatar(conversation),
            t.div(
                { className: "conversation-item-content" },
                t.div(
                    { className: "conversation-item-top" },
                    t.span({ className: "conversation-item-name" }, conversationName(conversation)),
                    t.span({ className: "conversation-item-time" }, formatListTime(conversation.lastAt)),
                ),
                t.div(
                    { className: "conversation-item-preview" },
                    status
                        ? t.i({
                            className: `${status.icon} conversation-status-${last.status}`,
                            ariaLabel: status.label,
                        })
                        : null,
                    t.span(null, last ? previewSender(conversation, last) + messagePreview(last) : ""),
                ),
            ),
        );
    }

    function threadHeader() {
        return t.header(
            { className: "conversation-thread-header" },
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent circle conversation-back",
                    ariaLabel: app.attrs.tooltip("Back to conversations"),
                    onclick: closeConversation,
                },
                t.i({ className: "ri-arrow-left-line", ariaHidden: true }),
            ),
            () => avatar(data.selected),
            t.div(
                { className: "conversation-thread-title" },
                t.div({ className: "txt-bold txt-ellipsis" }, () => conversationName(data.selected)),
                t.div({ className: "txt-sm txt-hint txt-ellipsis" }, () => conversationSubtitle(data.selected)),
            ),
            t.div({ className: "flex-fill" }),
            app.components.refreshButton({
                className: "btn sm transparent secondary circle",
                onclick: () => loadMessages(),
            }),
        );
    }

    function renderMessages(messages, conversation) {
        const isGroup = conversation?.threadType === "group";
        const result = [];
        let lastDay = "";

        for (const message of messages) {
            const day = dayKey(message.created);
            if (day !== lastDay) {
                lastDay = day;
                result.push(t.div({ className: "conversation-day" }, t.span(null, formatDay(message.created))));
            }
            result.push(messageRow(message, isGroup));
        }

        return result;
    }

    function messageRow(message, isGroup) {
        const status = message.source === "outbox" ? outboxStatuses[message.status] : null;
        const canReply = message.source === "message" && !message.recalled;

        return t.div(
            {
                className: `conversation-message ${message.isSelf ? "self" : "other"} ${
                    message.status === "failed" ? "failed" : ""
                }`,
            },
            isGroup && !message.isSelf && message.senderName
                ? t.div({ className: "conversation-message-sender" }, message.senderName)
                : null,
            t.div(
                { className: "conversation-message-row" },
                t.div(
                    { className: "conversation-bubble" },
                    ...messageBody(message),
                    t.div(
                        { className: "conversation-message-meta" },
                        t.span({ title: app.utils.toLocalDatetime(message.created) }, formatTime(message.created)),
                        status
                            ? t.span(
                                { className: `conversation-status conversation-status-${message.status}` },
                                t.i({ className: status.icon, ariaHidden: true }),
                                t.span({ className: "txt" }, status.label),
                            )
                            : null,
                    ),
                    message.status === "failed" && message.error
                        ? t.div({ className: "conversation-message-error" }, message.error)
                        : null,
                ),
                canReply
                    ? t.button(
                        {
                            type: "button",
                            className: "btn xs secondary transparent circle conversation-reply-btn",
                            ariaLabel: app.attrs.tooltip("Reply"),
                            onclick: () => startReply(message),
                        },
                        t.i({ className: "ri-reply-line", ariaHidden: true }),
                    )
                    : null,
            ),
        );
    }

    function composer() {
        return t.div(
            { className: "conversation-composer" },
            () => {
                if (!data.replyTo) {
                    return null;
                }
                return t.div(
                    { className: "conversation-reply-preview" },
                    t.i({ className: "ri-reply-line", ariaHidden: true }),
                    t.div(
                        { className: "conversation-reply-preview-content" },
                        t.div({ className: "txt-bold txt-sm" }, `Replying to ${data.replyTo.senderName || "message"}`),
                        t.div({ className: "txt-sm txt-hint txt-ellipsis" }, messagePreview(data.replyTo)),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn xs secondary transparent circle",
                            ariaLabel: app.attrs.tooltip("Cancel reply"),
                            onclick: () => (data.replyTo = null),
                        },
                        t.i({ className: "ri-close-line", ariaHidden: true }),
                    ),
                );
            },
            t.form(
                {
                    className: "conversation-composer-form",
                    onsubmit: (e) => {
                        e.preventDefault();
                        send();
                    },
                },
                t.textarea({
                    rows: 1,
                    maxLength: 5000,
                    placeholder: "Write a message... (Enter to send, Shift+Enter for a new line)",
                    ariaLabel: "Message",
                    value: () => data.draft,
                    onmount: (el) => (composerEl = el),
                    onunmount: () => (composerEl = null),
                    oninput: (e) => (data.draft = e.target.value),
                    onkeydown: (e) => {
                        // e.isComposing - don't send while an IME (e.g. Vietnamese Telex) is composing
                        if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
                            e.preventDefault();
                            send();
                        }
                    },
                }),
                t.button(
                    {
                        type: "submit",
                        className: () => `btn circle ${data.isSending ? "loading" : ""}`,
                        disabled: () => !data.draft.trim() || data.isSending,
                        ariaLabel: app.attrs.tooltip("Send"),
                    },
                    t.i({ className: "ri-send-plane-2-line", ariaHidden: true }),
                ),
            ),
        );
    }
}

// -------------------------------------------------------------------
// New message modal
// -------------------------------------------------------------------

const recipientTypeOptions = [
    { value: "phone", label: "Phone number" },
    { value: "user", label: "Zalo user ID" },
    { value: "group", label: "Zalo group ID" },
];

export function openZaloNewMessageModal(options = {}) {
    const formId = "zalo_new_message_" + app.utils.randomString();
    const form = store({
        recipientType: "phone",
        recipient: "",
        text: "",
        errors: {},
        isSending: false,
    });

    let modal;

    async function submit() {
        if (form.isSending) {
            return;
        }

        form.isSending = true;
        form.errors = {};

        const recipient = form.recipient.trim();
        const body = { text: form.text };
        if (form.recipientType === "phone") {
            body.phone = recipient;
        } else {
            body.threadId = recipient;
            body.threadType = form.recipientType;
        }

        try {
            const message = await app.pb.send("/api/conversations/zalo/messages", {
                method: "POST",
                body,
                requestKey: null,
            });
            app.toasts.success("The message was queued for sending.");
            app.modals.close(modal, true);
            options.onsent?.(message);
        } catch (err) {
            if (!err?.isAbort) {
                const fieldErrors = err?.response?.data || {};
                form.errors = {
                    recipient: (fieldErrors.phone || fieldErrors.threadId)?.message || "",
                    text: fieldErrors.text?.message || "",
                };
                if (!form.errors.recipient && !form.errors.text) {
                    app.toasts.error(apiErrorMessage(err, "Failed to send the message."));
                }
            }
        }

        form.isSending = false;
    }

    modal = t.div(
        {
            className: "modal popup sm zalo-new-message-modal",
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto" }, "New Zalo message"),
        ),
        t.form(
            {
                id: formId,
                className: "modal-content",
                onsubmit: (e) => {
                    e.preventDefault();
                    submit();
                },
            },
            t.div(
                { className: "field" },
                t.label({ htmlFor: formId + "_type" }, "Send to"),
                app.components.select({
                    id: formId + "_type",
                    value: () => form.recipientType,
                    options: recipientTypeOptions,
                    onchange: (selected) => {
                        form.recipientType = selected?.[0]?.value || "phone";
                        form.errors = {};
                    },
                }),
            ),
            t.div(
                { className: () => `field ${form.errors.recipient ? "error" : ""}` },
                t.label(
                    { htmlFor: formId + "_recipient" },
                    () => recipientTypeOptions.find((item) => item.value === form.recipientType)?.label,
                ),
                t.input({
                    id: formId + "_recipient",
                    type: () => form.recipientType === "phone" ? "tel" : "text",
                    required: true,
                    autofocus: true,
                    placeholder: () => form.recipientType === "phone" ? "e.g. 0901234567" : "",
                    value: () => form.recipient,
                    oninput: (e) => (form.recipient = e.target.value),
                }),
                () =>
                    form.errors.recipient
                        ? t.div({ className: "field-error txt-danger m-t-sm" }, form.errors.recipient)
                        : null,
            ),
            t.div(
                { className: "txt-sm txt-hint m-b-sm" },
                () =>
                    form.recipientType === "phone"
                        ? "The person must allow finding them by phone number in their Zalo privacy settings."
                        : "The id of the user or group as reported by the bridge (thread id).",
            ),
            t.div(
                { className: () => `field ${form.errors.text ? "error" : ""}` },
                t.label({ htmlFor: formId + "_text" }, "Message"),
                t.textarea({
                    id: formId + "_text",
                    rows: 4,
                    maxLength: 5000,
                    required: true,
                    value: () => form.text,
                    oninput: (e) => (form.text = e.target.value),
                }),
                () => form.errors.text ? t.div({ className: "field-error txt-danger m-t-sm" }, form.errors.text) : null,
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "submit",
                    "html-form": formId,
                    className: () => `btn ${form.isSending ? "loading" : ""}`,
                    disabled: () => form.isSending,
                },
                t.i({ className: "ri-send-plane-2-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Send"),
            ),
        ),
    );

    document.body.appendChild(modal);
    app.modals.open(modal);
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

function conversationName(conversation) {
    if (!conversation) {
        return "";
    }
    if (conversation.name) {
        return conversation.name;
    }
    return conversation.threadType === "group" ? `Group ${conversation.id}` : conversation.id;
}

function conversationSubtitle(conversation) {
    if (!conversation) {
        return "";
    }
    if (conversation.id.startsWith(phoneThreadPrefix)) {
        return "Phone number · waiting for the bridge to find the Zalo account";
    }

    const parts = [conversation.threadType === "group" ? "Group" : "User", conversation.id];
    if (conversation.phone) {
        parts.push(conversation.phone);
    }
    return parts.join(" · ");
}

function avatar(conversation) {
    const isGroup = conversation?.threadType === "group";
    const isPhone = conversation?.id?.startsWith(phoneThreadPrefix);
    const name = conversationName(conversation);

    // initials only for real names (not for ids/phone numbers)
    if (isGroup || isPhone || !/\p{L}/u.test(name)) {
        return t.div(
            { className: `conversation-avatar ${isGroup ? "group" : ""}`, ariaHidden: true },
            t.i({ className: isGroup ? "ri-group-line" : isPhone ? "ri-phone-line" : "ri-user-line" }),
        );
    }

    const initials = name
        .split(/\s+/)
        .filter(Boolean)
        .slice(0, 2)
        .map((part) => part[0])
        .join("")
        .toUpperCase();

    return t.div({ className: "conversation-avatar", ariaHidden: true }, initials);
}

function messageBody(message) {
    if (message.recalled) {
        return [t.div({ className: "conversation-message-text recalled" }, "Message recalled")];
    }

    const parts = [];

    const quote = quoteParts(message.quote);
    if (quote) {
        parts.push(
            t.div(
                { className: "conversation-quote" },
                quote.sender ? t.div({ className: "conversation-quote-sender" }, quote.sender) : null,
                t.div({ className: "conversation-quote-text" }, quote.text || "Attachment"),
            ),
        );
    }

    const content = message.content && typeof message.content == "object" ? message.content : null;
    const attachment = content ? attachmentPart(content) : null;
    if (attachment) {
        parts.push(attachment);
    }

    const text = message.text || (typeof message.content == "string" ? message.content : "");
    if (text) {
        parts.push(t.div({ className: "conversation-message-text" }, text));
    }

    if (message.attachments?.length) {
        parts.push(
            t.div(
                { className: "conversation-message-files" },
                ...message.attachments.map((name) =>
                    t.span({ className: "label" }, t.i({ className: "ri-attachment-2", ariaHidden: true }), name)
                ),
            ),
        );
    }

    if (!parts.length) {
        parts.push(t.div({ className: "conversation-message-text hint" }, `[${messageTypeLabel(message.msgType)}]`));
    }

    return parts;
}

function attachmentPart(content) {
    const href = safeURL(content.href);
    const thumb = safeURL(content.thumb) || (/\.(png|jpe?g|gif|webp)(\?|$)/i.test(href) ? href : "");
    const title = typeof content.title == "string" ? content.title : "";

    if (thumb) {
        return t.a(
            { href: href || thumb, target: "_blank", rel: "noopener noreferrer", className: "conversation-media" },
            t.img({ src: thumb, alt: title || "Image", loading: "lazy", referrerPolicy: "no-referrer" }),
        );
    }

    if (href) {
        return t.a(
            { href, target: "_blank", rel: "noopener noreferrer", className: "conversation-link" },
            t.i({ className: "ri-attachment-2", ariaHidden: true }),
            t.span({ className: "txt" }, title || href),
        );
    }

    return null;
}

function quoteParts(quote) {
    if (!quote || typeof quote != "object") {
        return null;
    }

    let text = quote.msg;
    if (text && typeof text == "object") {
        text = text.title || text.description || "";
    }

    return {
        sender: typeof quote.fromD == "string" ? quote.fromD : "",
        text: typeof text == "string" ? text : "",
    };
}

function previewSender(conversation, message) {
    if (message.isSelf) {
        return "You: ";
    }
    if (conversation.threadType === "group" && message.senderName) {
        return `${message.senderName}: `;
    }
    return "";
}

function messagePreview(message) {
    if (!message) {
        return "";
    }
    if (message.recalled) {
        return "Message recalled";
    }

    const text = message.text || (typeof message.content == "string" ? message.content : "");
    if (text) {
        return text.replace(/\s+/g, " ").trim();
    }

    if (message.content?.title && typeof message.content.title == "string") {
        return message.content.title;
    }

    return `[${messageTypeLabel(message.msgType)}]`;
}

function messageTypeLabel(msgType) {
    const type = String(msgType || "").toLowerCase();
    if (type.includes("photo") || type.includes("image")) {
        return "Photo";
    }
    if (type.includes("sticker")) {
        return "Sticker";
    }
    if (type.includes("video")) {
        return "Video";
    }
    if (type.includes("voice")) {
        return "Voice message";
    }
    if (type.includes("file") || type.includes("share")) {
        return "File";
    }
    if (type.includes("link")) {
        return "Link";
    }
    return msgType || "Message";
}

function safeURL(value) {
    if (typeof value != "string" || !/^https?:\/\//i.test(value)) {
        return "";
    }
    return value;
}

function parseDate(value) {
    if (!value) {
        return null;
    }
    const date = new Date(String(value).replace(" ", "T"));
    return isNaN(date.getTime()) ? null : date;
}

function dayKey(value) {
    const date = parseDate(value);
    return date ? date.toDateString() : "";
}

function formatTime(value) {
    const date = parseDate(value);
    return date ? date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" }) : "";
}

function formatDay(value) {
    const date = parseDate(value);
    if (!date) {
        return "";
    }

    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(today.getDate() - 1);

    if (date.toDateString() === today.toDateString()) {
        return "Today";
    }
    if (date.toDateString() === yesterday.toDateString()) {
        return "Yesterday";
    }

    return date.toLocaleDateString(undefined, {
        weekday: "short",
        day: "numeric",
        month: "short",
        year: date.getFullYear() === today.getFullYear() ? undefined : "numeric",
    });
}

function formatListTime(value) {
    const date = parseDate(value);
    if (!date) {
        return "";
    }
    if (date.toDateString() === new Date().toDateString()) {
        return formatTime(value);
    }
    return date.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

function apiErrorMessage(err, fallback) {
    const data = err?.response?.data || {};
    const fieldMessage = Object.values(data).find((item) => item?.message)?.message;
    return fieldMessage || err?.response?.message || fallback;
}
