import React, {useCallback, useEffect, useMemo, useState} from "react";
import modsResource from "../../../../../api/resources/mods";
import Button from "../../../../components/Button";
import Tab from "../../../../components/Tabs/Tab";
import TabControl from "../../../../components/Tabs/TabControl";
import ModSettingsSection from "./ModSettingsSection";
import {
    formatNodeValue,
    isEditable,
    nodeValidationError,
    nodesEqual,
    toEditableNode,
    toWireNode
} from "./modSettingValues";

const sections = [
    {key: "startup", title: "Startup"},
    {key: "runtime-global", title: "Map"},
];

const editableSections = settings => {
    const result = {};
    sections.forEach(({key}) => {
        result[key] = {};
        Object.entries(settings.sections?.[key] || {}).forEach(([name, node]) => {
            result[key][name] = toEditableNode(node);
        });
    });
    return result;
};

const ModSettings = () => {
    const [original, setOriginal] = useState(null);
    const [draft, setDraft] = useState({});
    const [version, setVersion] = useState(null);
    const [loadError, setLoadError] = useState(null);
    const [isLoading, setIsLoading] = useState(true);
    const [isSaving, setIsSaving] = useState(false);

    const load = useCallback(() => {
        setIsLoading(true);
        modsResource.settings.get()
            .then(settings => {
                setOriginal(editableSections(settings));
                setVersion(settings.version);
                setDraft({});
                setLoadError(null);
            })
            .catch(error => {
                setOriginal(null);
                setDraft({});
                setLoadError(error?.response?.data || "Could not load mod-settings.dat.");
            })
            .finally(() => setIsLoading(false));
    }, []);

    useEffect(() => {
        load();
    }, [load]);

    const updateNode = (section, name, node) => {
        setDraft(previous => ({
            ...previous,
            [section]: {...(previous[section] || {}), [name]: node},
        }));
    };

    const revertNode = (section, name) => {
        setDraft(previous => {
            const sectionDraft = {...(previous[section] || {})};
            delete sectionDraft[name];
            return {...previous, [section]: sectionDraft};
        });
    };

    const changes = useMemo(() => {
        if (!original) {
            return [];
        }

        const list = [];
        Object.entries(draft).forEach(([section, nodes]) => {
            Object.entries(nodes).forEach(([name, node]) => {
                const originalNode = original[section]?.[name];
                if (originalNode && !nodesEqual(node, originalNode)) {
                    list.push({section, name, type: node.type, value: toWireNode(node).value});
                }
            });
        });
        return list;
    }, [draft, original]);

    const entriesBySection = useMemo(() => {
        const result = {};
        sections.forEach(({key}) => {
            result[key] = Object.entries(original?.[key] || {})
                .map(([name, originalNode]) => {
                    const draftNode = draft[key]?.[name];
                    const node = draftNode ?? originalNode;
                    return {
                        name,
                        node,
                        type: node.type,
                        readOnly: !isEditable(node),
                        modified: draftNode !== undefined && !nodesEqual(node, originalNode),
                        originalDisplay: formatNodeValue(originalNode),
                        error: nodeValidationError(node),
                    };
                })
                .sort((left, right) => left.name.localeCompare(right.name));
        });
        return result;
    }, [draft, original]);

    const invalidCount = useMemo(() => {
        let count = 0;
        Object.values(entriesBySection).forEach(entries => {
            entries.forEach(entry => {
                if (entry.error) {
                    count++;
                }
            });
        });
        return count;
    }, [entriesBySection]);

    const save = async () => {
        if (changes.length === 0 || invalidCount > 0) {
            return;
        }

        setIsSaving(true);
        try {
            const result = await modsResource.settings.update(changes);
            window.flash(`Saved ${result.changes} mod setting${result.changes === 1 ? "" : "s"}`, "green");
            load();
        } catch (error) {
            // The API client already flashed the error message.
        } finally {
            setIsSaving(false);
        }
    };

    if (isLoading) {
        return <div className="text-white">Loading mod settings...</div>;
    }

    if (!original) {
        return (
            <div className="text-white">
                <div className="text-red font-bold text-xl mb-2">{loadError}</div>
                <div className="mb-4">
                    mod-settings.dat is stored in the mods directory. Upload your settings with the
                    "Upload Mod" tab and reload this tab.
                </div>
                <Button size="sm" onClick={load}>Retry</Button>
            </div>
        );
    }

    return (
        <div>
            <div className="flex flex-wrap items-center mb-4">
                <div className="mr-auto mb-2">
                    {changes.length > 0
                        ? <span className="text-orange font-bold">
                            {changes.length} unsaved change{changes.length === 1 ? "" : "s"}
                          </span>
                        : <span className="text-white">All settings match mod-settings.dat.</span>}
                    {invalidCount > 0
                        ? <span className="text-red ml-3">
                            {invalidCount} invalid value{invalidCount === 1 ? "" : "s"}
                          </span>
                        : null}
                </div>
                <Button size="sm" className="mr-2 mb-2" isLoading={isSaving}
                        isDisabled={changes.length === 0 || invalidCount > 0}
                        onClick={save}>Save</Button>
                <Button size="sm" type="danger" className="mb-2" isLoading={isSaving}
                        isDisabled={changes.length === 0}
                        onClick={() => setDraft({})}>Discard</Button>
            </div>

            <div className="text-gray-light text-sm mb-4">
                The file version is {version}. Changes are written to mod-settings.dat and applied when the
                server restarts. The per-player settings section is not shown and is left untouched.
            </div>

            <TabControl>
                {sections.map(({key, title}) => (
                    <Tab key={key} title={title}>
                        <ModSettingsSection
                            entries={entriesBySection[key] || []}
                            onChange={(name, node) => updateNode(key, name, node)}
                            onRevert={name => revertNode(key, name)}
                        />
                    </Tab>
                ))}
            </TabControl>
        </div>
    );
};

export default ModSettings;
