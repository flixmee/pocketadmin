window.app = window.app || {};
window.app.components = window.app.components || {};
window.app.utils = window.app.utils || {};

const automationI18nTriggerTypes = [
    "i18n.translation_missing",
    "i18n.locale_published",
    "i18n.translation_updated",
    "i18n.ai_translation_finished",
];
const automationRecordTriggerTypes = [
    "record.beforeCreate",
    "record.beforeUpdate",
    "record.create",
    "record.update",
    "record.delete",
];

const automationMappingBaseGroups = [
    {
        title: "Trigger",
        tokens: [
            "{{trigger.type}}",
            "{{trigger.collectionId}}",
            "{{trigger.collectionName}}",
            "{{automation.id}}",
            "{{automation.name}}",
            "{{run.id}}",
            "{{run.status}}",
        ],
    },
    {
        title: "Record",
        tokens: ["{{record.id}}", "{{record.*}}", "{{recordOriginal.*}}"],
        triggerTypes: [...automationRecordTriggerTypes, ...automationI18nTriggerTypes],
    },
    {
        title: "Webhook",
        tokens: [
            "{{request.method}}",
            "{{request.path}}",
            "{{request.remoteIP}}",
            "{{request.headers.*}}",
            "{{request.query.*}}",
            "{{request.body.*}}",
            "{{request.resume.*}}",
        ],
        triggerTypes: ["webhook"],
    },
    {
        title: "Telegram",
        tokens: [
            "{{telegram.updateId}}",
            "{{telegram.message.text}}",
            "{{telegram.text}}",
            "{{telegram.chat.id}}",
            "{{telegram.chat.username}}",
            "{{telegram.from.id}}",
            "{{telegram.from.username}}",
            "{{telegram.update.*}}",
        ],
        triggerTypes: ["telegram.message"],
    },
    {
        title: "Zalo",
        tokens: [
            "{{zalo.text}}",
            "{{zalo.threadId}}",
            "{{zalo.threadType}}",
            "{{zalo.senderId}}",
            "{{zalo.senderName}}",
            "{{zalo.messageId}}",
            "{{zalo.msgId}}",
            "{{zalo.msgType}}",
            "{{zalo.ts}}",
            "{{zalo.content.*}}",
            "{{zalo.message.*}}",
        ],
        triggerTypes: ["zalo.message"],
    },
    {
        title: "Input",
        tokens: [
            "{{input.*}}",
            "{{trigger.caller.automationId}}",
            "{{trigger.caller.automationName}}",
            "{{trigger.caller.runId}}",
        ],
        triggerTypes: ["manual"],
    },
    {
        title: "i18n",
        tokens: [
            "{{i18n.collectionId}}",
            "{{i18n.collectionName}}",
            "{{i18n.groupId}}",
            "{{i18n.locale}}",
            "{{i18n.sourceLocale}}",
            "{{i18n.sourceRecordId}}",
            "{{i18n.targetLocale}}",
            "{{i18n.targetRecordId}}",
            "{{i18n.missingLocales}}",
            "{{i18n.translationTotal}}",
            "{{i18n.translationJobId}}",
            "{{i18n.provider}}",
            "{{i18n.model}}",
        ],
        triggerTypes: automationI18nTriggerTypes,
    },
    {
        title: "Steps",
        tokens: [
            "{{prevStep.status}}",
            "{{prevStep.output.*}}",
            "{{prevStep.error}}",
            "{{steps[0].status}}",
            "{{steps[0].output.*}}",
            "{{steps[0].error}}",
        ],
    },
];

// trigger payload roots collected from the recent runs and the trigger types
// they belong to (used to hide stale paths after the trigger type was changed)
const automationRunSampleRoots = {
    request: ["webhook"],
    telegram: ["telegram.message"],
    zalo: ["zalo.message"],
    i18n: automationI18nTriggerTypes,
    input: null,
    record: [...automationRecordTriggerTypes, ...automationI18nTriggerTypes],
    recordOriginal: [...automationRecordTriggerTypes, ...automationI18nTriggerTypes],
};
const automationRunSampleMaxPaths = 400;
const automationRunSampleMaxDepth = 6;
const automationRunSampleMaxKeys = 50;
const automationRunSampleMaxLength = 60;
const automationRunSampleSensitiveKey = /authorization|cookie|password|secret|token|api[-_]?key/i;
const automationIdentifierPattern = /^[a-zA-Z_$][a-zA-Z0-9_$]*$/;

// data paths (and a sample value for each) collected from the recent runs
// of the automation that is currently being edited
const automationRunSamples = store({
    runsCount: 0,
    paths: [],
});
let automationRunSampleValues = new Map();

/**
 * Creates a CodeEditor-powered input for automation template fields.
 * Typing `{{` opens data mapping autocomplete suggestions.
 *
 * @param  {Object} [propsArg]
 * @return {Element}
 */
window.app.components.automationInput = function(propsArg = {}) {
    const externalAutocomplete = propsArg.autocomplete;

    return app.components.codeEditor({
        ...propsArg,
        className: () =>
            ["automation-input", resolveProp(propsArg.className)]
                .filter(Boolean)
                .join(" "),
        autocomplete: (word) => {
            const suggestions = app.utils.automationMappingAutocomplete(word, {
                triggerType: resolveProp(propsArg.triggerType),
                triggerCollectionRef: resolveProp(propsArg.triggerCollectionRef),
                steps: resolveProp(propsArg.steps),
                maxItems: resolveProp(propsArg.maxAutocompleteItems),
            });

            if (typeof externalAutocomplete == "function") {
                return mergeAutocompleteSuggestions(suggestions, externalAutocomplete(word) || []);
            }

            return suggestions;
        },
    });
};

window.app.utils.automationMappingTokenGroups = function(options = {}) {
    const triggerType = resolveProp(options.triggerType);
    const triggerCollectionRef = resolveProp(options.triggerCollectionRef);
    const result = automationMappingBaseGroups
        .filter((group) => !group.triggerTypes || group.triggerTypes.includes(triggerType))
        .map((group) => ({
            ...group,
            tokens: [...group.tokens],
        }));

    // the automation editor registers the input fields that are currently being edited
    if (triggerType === "manual" && typeof app.utils.automationEditorManualInputNames === "function") {
        const inputGroup = result.find((group) => group.title === "Input");
        const names = app.utils.automationEditorManualInputNames() || [];
        if (inputGroup && names.length) {
            inputGroup.tokens.unshift(...names.map((name) => `{{input.${name}}}`));
        }
    }

    const recordTokens = collectionMappingTokens("record", triggerCollectionRef);
    if (recordTokens.length) {
        result.push({
            title: "Record fields",
            tokens: recordTokens,
            triggerTypes: [...automationRecordTriggerTypes, ...automationI18nTriggerTypes],
        });
    }

    if (triggerType === "record.beforeUpdate" || triggerType === "record.update" || triggerType === "record.delete") {
        const originalTokens = collectionMappingTokens("recordOriginal", triggerCollectionRef);
        if (originalTokens.length) {
            result.push({
                title: "Original fields",
                tokens: originalTokens,
                triggerTypes: ["record.beforeUpdate", "record.update", "record.delete"],
            });
        }
    }

    const stepTokens = stepMappingTokens(resolveProp(options.steps));
    if (stepTokens.length) {
        result.push({
            title: "Configured steps",
            tokens: stepTokens,
        });
    }

    const runTokens = runSampleMappingTokens(triggerType);
    if (runTokens.length) {
        result.push({
            title: "Recent runs",
            description:
                `Paths found in the last ${automationRunSamples.runsCount} run(s). Hover a token to see its last value.`,
            tokens: runTokens,
            isRunSample: true,
        });
    }

    return result;
};

/**
 * Collects the template data paths of the specified runs so that they
 * can be suggested by the template autocomplete and the mapping palette.
 *
 * Newer runs should be first - their values are used as samples.
 *
 * @param {Array} runs
 */
window.app.utils.setAutomationRunSamples = function(runs) {
    runs = Array.isArray(runs) ? runs : [];

    const samples = new Map();
    for (const run of runs) {
        const payload = parseRunSampleJSON(run?.input);
        if (payload && typeof payload == "object" && !Array.isArray(payload)) {
            collectRunSamplePaths(samples, "trigger.type", payload.triggerType || run.triggerType);
            collectRunSamplePaths(samples, "trigger.collectionId", payload.collectionId || undefined);
            collectRunSamplePaths(samples, "trigger.collectionName", payload.collectionName || undefined);
            collectRunSamplePaths(samples, "trigger.caller", payload.caller);
            for (const root of Object.keys(automationRunSampleRoots)) {
                collectRunSamplePaths(samples, root, payload[root]);
            }
        }

        const stepResults = parseRunSampleJSON(run?.stepResults);
        if (Array.isArray(stepResults)) {
            stepResults.forEach((result, i) => {
                collectRunSamplePaths(samples, `steps.${i}.status`, result?.status);
                collectRunSamplePaths(samples, `steps.${i}.output`, result?.output);
                collectRunSamplePaths(samples, `steps.${i}.error`, result?.error || undefined);
            });
        }
    }

    automationRunSampleValues = samples;
    automationRunSamples.runsCount = samples.size ? runs.length : 0;
    automationRunSamples.paths = [...samples.keys()];
};

/**
 * Returns the last seen value of the specified mapping token (if any).
 *
 * @param  {string} token
 * @return {string|undefined}
 */
window.app.utils.automationRunSampleValue = function(token) {
    return automationRunSampleValues.get(tokenPath(token));
};

window.app.utils.automationMappingTokens = function(options = {}) {
    const tokens = [];
    const seen = new Set();

    for (const group of app.utils.automationMappingTokenGroups(options)) {
        for (const token of group.tokens || []) {
            if (!token || seen.has(token)) {
                continue;
            }
            seen.add(token);
            tokens.push({
                value: token,
                label: `${token} · ${group.title}`,
                sample: app.utils.automationRunSampleValue(token),
            });
        }
    }

    return tokens;
};

window.app.utils.automationMappingAutocomplete = function(word, options = {}) {
    const templateMatch = templateAutocompleteMatch(word);
    if (!templateMatch) {
        return [];
    }

    const normalizedQuery = normalizeTokenExpression(templateMatch.query);
    const tokens = app.utils.automationMappingTokens(options);
    const matches = [];

    for (const token of tokens) {
        const expression = normalizeTokenExpression(token.value);
        if (
            normalizedQuery
            && !expression.includes(normalizedQuery)
        ) {
            continue;
        }

        matches.push({
            value: templateMatch.prefix + token.value,
            label: token.sample === undefined ? token.label : () => autocompleteSampleLabel(token),
            _startsWith: normalizedQuery && expression.startsWith(normalizedQuery) ? 0 : 1,
            // prefer the paths that were seen in the recent runs
            _hasSample: token.sample === undefined ? 1 : 0,
            _length: expression.length,
        });
    }

    matches.sort((a, b) => {
        if (a._startsWith !== b._startsWith) {
            return a._startsWith - b._startsWith;
        }
        if (a._hasSample !== b._hasSample) {
            return a._hasSample - b._hasSample;
        }
        return a._length - b._length;
    });

    const maxItems = Number(options.maxItems) > 0 ? Number(options.maxItems) : 60;
    return matches.slice(0, maxItems).map(({ _startsWith, _hasSample, _length, ...item }) => item);
};

function autocompleteSampleLabel(token) {
    return t.span(
        { className: "automation-autocomplete-item" },
        t.span({ className: "automation-autocomplete-token" }, token.label),
        t.span({ className: "automation-autocomplete-sample" }, token.sample),
    );
}

function runSampleMappingTokens(triggerType) {
    const tokens = [];
    for (const path of automationRunSamples.paths) {
        const triggerTypes = automationRunSampleRoots[path.split(/[.[]/)[0]];
        if (triggerType && triggerTypes && !triggerTypes.includes(triggerType)) {
            continue;
        }
        tokens.push(`{{${path}}}`);
    }
    return tokens;
}

function collectRunSamplePaths(samples, path, value, depth = 0) {
    if (value === undefined || samples.size >= automationRunSampleMaxPaths) {
        return;
    }

    if (!samples.has(path)) {
        samples.set(path, formatRunSampleValue(path, value));
    }

    if (depth >= automationRunSampleMaxDepth || value === null || typeof value != "object") {
        return;
    }

    if (Array.isArray(value)) {
        if (value.length) {
            // dot indexes are resolved without a JS VM but can't follow a bracket key
            const indexPath = path.includes("[") ? `${path}[0]` : `${path}.0`;
            collectRunSamplePaths(samples, indexPath, value[0], depth + 1);
        }
        return;
    }

    for (const key of Object.keys(value).slice(0, automationRunSampleMaxKeys)) {
        let keyPath;
        if (automationIdentifierPattern.test(key)) {
            keyPath = `${path}.${key}`;
        } else {
            // the path is evaluated as JS expression -> convert the dot indexes too
            // (a path without brackets can't contain quoted keys)
            const jsPath = path.includes("[") ? path : path.replace(/\.(\d+)(?=\.|$)/g, "[$1]");
            keyPath = `${jsPath}[${JSON.stringify(key)}]`;
        }
        collectRunSamplePaths(samples, keyPath, value[key], depth + 1);
    }
}

function formatRunSampleValue(path, value) {
    if (value === null) {
        return "null";
    }

    if (Array.isArray(value)) {
        return value.length == 1 ? "[1 item]" : `[${value.length} items]`;
    }

    if (typeof value == "object") {
        const total = Object.keys(value).length;
        return total == 1 ? "{1 key}" : `{${total} keys}`;
    }

    if (typeof value == "string" && automationRunSampleSensitiveKey.test(path.split(/[.[]/).pop())) {
        return "••••••";
    }

    const result = typeof value == "string" ? JSON.stringify(value) : String(value);
    if (result.length > automationRunSampleMaxLength) {
        return result.substring(0, automationRunSampleMaxLength - 1) + "…";
    }

    return result;
}

function parseRunSampleJSON(value) {
    if (typeof value != "string") {
        return value;
    }

    try {
        return JSON.parse(value);
    } catch (_) {
        return null;
    }
}

function tokenPath(token) {
    token = String(token || "").trim();
    if (token.startsWith("{{") && token.endsWith("}}")) {
        token = token.substring(2, token.length - 2);
    }
    return token.trim();
}

function templateAutocompleteMatch(word) {
    word = typeof word == "string" ? word : "";
    const markerIndex = word.lastIndexOf("{{");
    if (markerIndex < 0) {
        return null;
    }

    return {
        prefix: word.substring(0, markerIndex),
        query: word.substring(markerIndex + 2).replace(/^\s+/, "").replace(/\}+$/g, ""),
    };
}

function normalizeTokenExpression(value) {
    value = String(value || "").trim();
    if (value.startsWith("{{")) {
        value = value.substring(2);
    }
    if (value.endsWith("}}")) {
        value = value.substring(0, value.length - 2);
    }
    return value.trim().toLowerCase();
}

function collectionMappingTokens(root, collectionIdOrName) {
    const collection = findAutomationCollection(collectionIdOrName);
    if (!collection) {
        return [];
    }

    return collectionDataPaths(collection).map((path) => `{{${root}.${path}}}`);
}

function collectionDataPaths(collection, prefix = "", level = 0, visited = new Set()) {
    if (!collection || level > 3) {
        return [];
    }

    const visitKey = `${collection.id || collection.name}:${level}:${prefix}`;
    if (visited.has(visitKey)) {
        return [];
    }
    visited.add(visitKey);

    const paths = [];
    pushUnique(paths, `${prefix}id`);

    for (const field of collection.fields || []) {
        if (shouldSkipAutomationField(collection, field)) {
            continue;
        }

        const fieldPath = `${prefix}${field.name}`;
        pushUnique(paths, fieldPath);

        if (field.type === "relation" && field.collectionId) {
            const relationCollection = findAutomationCollection(field.collectionId);
            for (const childPath of collectionDataPaths(relationCollection, `${fieldPath}.`, level + 1, visited)) {
                pushUnique(paths, childPath);
            }
        }
    }

    visited.delete(visitKey);
    return paths;
}

function stepMappingTokens(steps) {
    if (!Array.isArray(steps)) {
        return [];
    }

    const tokens = [];
    for (let i = 0; i < steps.length; i++) {
        pushUnique(tokens, `{{steps[${i}].status}}`);
        pushUnique(tokens, `{{steps[${i}].output.*}}`);
        pushUnique(tokens, `{{steps[${i}].error}}`);
    }
    return tokens;
}

function shouldSkipAutomationField(collection, field) {
    return !!(
        !field
        || field.type === "password"
        || (collection?.type === "auth" && field.name === "tokenKey")
    );
}

function findAutomationCollection(idOrName) {
    if (!idOrName) {
        return null;
    }

    return (app.store.collections || []).find((collection) => {
        return collection?.id === idOrName || collection?.name === idOrName;
    }) || null;
}

function mergeAutocompleteSuggestions(primary, secondary) {
    const result = [];
    const seen = new Set();

    for (const item of [...(primary || []), ...(secondary || [])]) {
        const value = typeof item == "object" ? item.value : item;
        if (!value || seen.has(value)) {
            continue;
        }
        seen.add(value);
        result.push(item);
    }

    return result;
}

function pushUnique(arr, value) {
    if (value && !arr.includes(value)) {
        arr.push(value);
    }
}

function resolveProp(value) {
    if (typeof value != "function") {
        return value;
    }

    try {
        return value();
    } catch (err) {
        console.warn("failed to resolve automation input prop", err);
        return undefined;
    }
}
