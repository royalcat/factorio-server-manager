import Panel from "../../components/Panel";
import React, {useEffect, useState} from "react";
import modsResource from "../../../api/resources/mods";
import Button from "../../components/Button";
import server from "../../../api/resources/server";
import TabControl from "../../components/Tabs/TabControl";
import Tab from "../../components/Tabs/Tab";
import AddMod from "./components/AddMod/AddMod";
import UploadMod from "./components/UploadMod";
import LoadMods from "./components/LoadMods";
import CreateModPack from "./components/CreateModPack";
import ModPack from "./components/ModPack";
import ModList from "./components/ModList";
import SearchMods from "./components/SearchMods/SearchMods";
import ImportModPack from "./components/ImportModPack";
import {coerce, gt, satisfies} from "semver";
import {formatFactorioVersion, formatFactorioVersionShort, formatModVersion} from "../../utils/version";

const releaseVersion = release => coerce(release.version);
const modVersion = mod => coerce(mod.version);
const factorioReleaseVersion = release => coerce(release.info_json.factorio_version);
const builtInMods = ["base", "elevated-rails", "quality", "space-age"];

const requiredDependencyName = dependency => {
    const normalized = dependency.trim();
    if (normalized === "") {
        return null;
    }

    const fields = normalized.split(/\s+/);
    const name = fields[0];
    if (["?", "!", "~", "(?)"].includes(name)) {
        return null;
    }
    if (name.startsWith("?") || name.startsWith("!") || name.startsWith("~")) {
        return null;
    }
    if (builtInMods.includes(name)) {
        return null;
    }

    return name;
};

const requiredDependencyNames = dependencies => (dependencies || [])
    .map(requiredDependencyName)
    .filter(Boolean);

const isReleaseCompatible = (release, factorioVersion) => {
    const requiredFactorioVersion = factorioReleaseVersion(release);
    const installedFactorioVersion = coerce(factorioVersion);
    if (!requiredFactorioVersion || !installedFactorioVersion) {
        return false;
    }

    // factorio_version requires exact major.minor match (patch ignored)
    if (installedFactorioVersion.major === requiredFactorioVersion.major &&
        installedFactorioVersion.minor === requiredFactorioVersion.minor &&
        installedFactorioVersion.major > 0) {
        return true;
    }

    // Special case: Factorio 1.0 is compatible with mods tagged 0.18
    return satisfies(installedFactorioVersion.version, "1.0.0") &&
        satisfies(requiredFactorioVersion, "0.18.x");
};

const releaseTimestamp = release => Date.parse(release.released_at || "") || 0;

const newestRelease = releases => releases.reduce((newest, release) => {
    const version = releaseVersion(release);
    if (!version) {
        return newest;
    }
    if (!newest || releaseTimestamp(release) > releaseTimestamp(newest)) {
        return release;
    }
    if (releaseTimestamp(release) === releaseTimestamp(newest) && gt(version, releaseVersion(newest))) {
        return release;
    }

    return newest;
}, null);

const buildModMetadata = (mod, portalInfo, factorioVersion) => {
    if (!portalInfo || portalInfo.error) {
        return {
            status: "unknown",
            reason: portalInfo?.error || "Portal metadata unavailable",
        };
    }

    const latestRelease = newestRelease(portalInfo.releases || []);
    const latestCompatibleRelease = newestRelease((portalInfo.releases || []).filter(release =>
        isReleaseCompatible(release, factorioVersion)
    ));
    const currentVersion = modVersion(mod);
    const latestVersion = latestRelease ? releaseVersion(latestRelease) : null;
    const latestCompatibleVersion = latestCompatibleRelease ? releaseVersion(latestCompatibleRelease) : null;

    let update = null;
    let status = "current";
    let reason = "";

    if (!latestRelease || !latestVersion || !currentVersion) {
        status = "unknown";
        reason = "No release metadata available";
    } else if (latestCompatibleRelease && latestCompatibleVersion && gt(latestCompatibleVersion, currentVersion)) {
        status = "update-available";
        update = {
            downloadUrl: latestCompatibleRelease.download_url,
            fileName: latestCompatibleRelease.file_name,
            modName: mod.name,
        };
    } else if (gt(latestVersion, currentVersion)) {
        status = "current";
        reason = `Latest requires Factorio ${formatFactorioVersionShort(latestRelease.info_json.factorio_version)}`;
    } else if (!mod.compatibility) {
        status = "current";
        reason = `Targets Factorio ${formatFactorioVersionShort(mod.factorio_version)}`;
    }

    return {
        status,
        reason,
        latestRelease,
        latestCompatibleRelease,
        latestVersion: formatModVersion(latestCompatibleRelease?.version || latestRelease?.version),
        latestReleasedAt: (latestCompatibleRelease || latestRelease)?.released_at,
        factorioVersion: formatFactorioVersionShort((latestCompatibleRelease || latestRelease)?.info_json?.factorio_version),
        update,
        changelogUrl: `https://mods.factorio.com/mod/${mod.name}/changelog`,
    };
};

const Mods = ({serverStatus}) => {

    const [installedMods, setInstalledMods] = useState([]);
    const [modPacks, setModPacks] = useState([])
    const [factorioVersion, setFactorioVersion] = useState(null);
    const [isDeletingAllMods, setIsDeletingAllMods] = useState(false);
    const [isUpdatingMods, setIsUpdatingMods] = useState(false);
    const [portalInfo, setPortalInfo] = useState({});
    const [selectedUpdates, setSelectedUpdates] = useState({});

    const fetchInstalledMods = () => {
        return modsResource.installed()
            .then(setInstalledMods);
    };

    const fetchModPacks = () => {
        modsResource.packs.list()
            .then(setModPacks)
    }

    const deleteAllMods = () => {
        setIsDeletingAllMods(true);
        modsResource.deleteAll()
            .then(fetchInstalledMods)
            .finally(() => setIsDeletingAllMods(false))
    }

    const metadataByMod = installedMods.reduce((metadata, mod) => {
        metadata[mod.name] = buildModMetadata(mod, portalInfo[mod.name], factorioVersion);
        return metadata;
    }, {});

    const compatibleUpdates = installedMods
        .map(mod => metadataByMod[mod.name]?.update)
        .filter(Boolean);
    const selectedUpdatePayloads = compatibleUpdates.filter(update => selectedUpdates[update.modName]);

    const updateMods = async updates => {
        setIsUpdatingMods(true);

        // Sequential: each update deletes the old zip then writes the new one
        // on the same disk directory. Parallel requests would race on file state.
        for (const update of updates) {
            try {
                await modsResource.update(update);
            } catch (e) {
                console.error(`Failed to update mod ${update.modName}:`, e);
            }
        }
        fetchInstalledMods();
        setIsUpdatingMods(false);
    }

    const updateAllMods = () => {
        updateMods(compatibleUpdates);
    }

    const updateSelectedMods = () => {
        updateMods(selectedUpdatePayloads);
    }

    const toggleSelectedUpdate = modName => {
        setSelectedUpdates(selected => ({
            ...selected,
            [modName]: !selected[modName]
        }));
    }

    useEffect(() => {
        server.factorioVersion()
            .then(data => {
                setFactorioVersion(data.base_mod_version)
                fetchInstalledMods();
                fetchModPacks();
            });
    }, []);

    useEffect(() => {
        if (factorioVersion === null || installedMods.length === 0) {
            return;
        }

        let canceled = false;
        Promise.all(installedMods.map(mod =>
            modsResource.portal.info(mod.name)
                .then(data => ({name: mod.name, data}))
                .catch(() => ({name: mod.name, data: {error: "Portal metadata unavailable"}}))
        )).then(results => {
            if (canceled) {
                return;
            }

            const info = {};
            results.forEach(result => {
                info[result.name] = result.data;
            });
            setPortalInfo(info);
        });

        return () => {
            canceled = true;
        };
    }, [installedMods, factorioVersion]);

    useEffect(() => {
        setSelectedUpdates(selected => {
            const next = {};
            compatibleUpdates.forEach(update => {
                next[update.modName] = selected[update.modName] || false;
            });
            return next;
        });
    }, [installedMods, portalInfo, factorioVersion]);

    const toggleMod = modName => {
        return modsResource
            .toggle(modName)
            .then(fetchInstalledMods)
    }

    const deleteMod = modName => {
        return modsResource
            .delete(modName)
            .then(fetchInstalledMods)
    }

    const updateMod = version => {
        return modsResource
            .update(version)
            .then(fetchInstalledMods)
    }

    let disabled = serverStatus.running

    return (
        <div>
            {disabled ?
                <Panel className="mb-6"
                       content={
                           <div className="text-red font-bold text-xl">
                               Changing mods is disabled while the server is running!
                           </div>
                       }
                />
                :
                <TabControl>
                    <Tab title="Install Mod">
                        <AddMod refetchInstalledMods={fetchInstalledMods}/>
                    </Tab>
                    <Tab title="Search">
                        <SearchMods factorioVersion={factorioVersion} refetchInstalledMods={fetchInstalledMods}/>
                    </Tab>
                    <Tab title="Upload Mod">
                        <UploadMod refetchInstalledMods={fetchInstalledMods}/>
                    </Tab>
                    <Tab title="Load Mods from Save">
                        <LoadMods refreshMods={fetchInstalledMods}/>
                    </Tab>
                </TabControl>
            }
            <Panel
                title="Mods"
                className="mb-6"
                content={
                    <ModList toggleMod={toggleMod}
                             updateMod={updateMod}
                             deleteMod={deleteMod}
                             mods={installedMods}
                             metadataByMod={metadataByMod}
                             selectedUpdates={selectedUpdates}
                             toggleSelectedUpdate={toggleSelectedUpdate}
                             factorioVersion={factorioVersion}
                             disabled={disabled}
                    />
                }
                actions={
                    <>
                        {
                            !disabled &&
                            <>
                                <Button size="sm" className="mr-2" type="danger" isLoading={isDeletingAllMods}
                                        onClick={deleteAllMods}>Delete all Mods</Button>
                                <Button size="sm" className="mr-2" isLoading={isUpdatingMods}
                                        isDisabled={compatibleUpdates.length === 0}
                                        onClick={updateAllMods}>Update all Mods</Button>
                                <Button size="sm" className="mr-2" isLoading={isUpdatingMods}
                                        isDisabled={selectedUpdatePayloads.length === 0}
                                        onClick={updateSelectedMods}>Update selected Mods</Button>
                            </>
                        }
                        <a className="bg-gray-light py-1 px-2 hover:glow-orange hover:bg-orange inline-block accentuated text-black font-bold"
                           href={modsResource.downloadAllURL}>Download all Mods</a>
                    </>
                }
            />

            <Panel
                title="Mod packs"
                className="mb-6"
                content={
                    modPacks.map(
                        (pack, i) =>
                            <ModPack factorioVersion={factorioVersion}
                                     key={i}
                                     modPack={pack}
                                     reloadMods={fetchInstalledMods}
                                     reloadModPacks={fetchModPacks}
                                     disabled={disabled}
                            />
                    )
                }
                actions={
                    <>
                        <CreateModPack onSuccess={fetchModPacks}/>
                        <ImportModPack onSuccess={fetchModPacks}/>
                    </>
                }
            />
        </div>
    )
}

export default Mods;
