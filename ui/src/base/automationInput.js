window.app = window.app || {};
window.app.components = window.app.components || {};
window.app.utils = window.app.utils || {};

const automationI18nTriggerTypes = [
    "i18n.translation_missing",
    "i18n.locale_published",
    "i18n.translation_updated",
    "i18n.ai_translation_finished",
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
        triggerTypes: ["record.create", "record.update", "record.delete", ...automationI18nTriggerTypes],
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

    const recordTokens = collectionMappingTokens("record", triggerCollectionRef);
    if (recordTokens.length) {
        result.push({
            title: "Record fields",
            tokens: recordTokens,
            triggerTypes: ["record.create", "record.update", "record.delete", ...automationI18nTriggerTypes],
        });
    }

    if (triggerType === "record.update" || triggerType === "record.delete") {
        const originalTokens = collectionMappingTokens("recordOriginal", triggerCollectionRef);
        if (originalTokens.length) {
            result.push({
                title: "Original fields",
                tokens: originalTokens,
                triggerTypes: ["record.update", "record.delete"],
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

    return result;
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
            ...token,
            value: templateMatch.prefix + token.value,
            _startsWith: normalizedQuery && expression.startsWith(normalizedQuery) ? 0 : 1,
            _length: expression.length,
        });
    }

    matches.sort((a, b) => {
        if (a._startsWith !== b._startsWith) {
            return a._startsWith - b._startsWith;
        }
        return a._length - b._length;
    });

    const maxItems = Number(options.maxItems) > 0 ? Number(options.maxItems) : 60;
    return matches.slice(0, maxItems).map(({ _startsWith, _length, ...item }) => item);
};

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
