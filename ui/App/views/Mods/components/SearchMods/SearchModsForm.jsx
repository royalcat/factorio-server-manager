import React, {useEffect, useState} from "react";
import modsResource from "../../../../../api/resources/mods";
import Button from "../../../../components/Button";
import Label from "../../../../components/Label";
import {useForm} from "react-hook-form";
import Input from "../../../../components/Input";
import {FontAwesomeIcon} from "@fortawesome/react-fontawesome";
import {faDownload, faExternalLinkAlt, faSpinner} from "@fortawesome/free-solid-svg-icons";
import SelectVersionForm from "../AddMod/components/SelectVersionForm";

const LinkModPortal = () => {
    return <a href="https://mods.factorio.com" target="_blank" rel="noopener noreferrer" className="px-2 text-blue hover:text-blue-light">Mod Portal <FontAwesomeIcon icon={faExternalLinkAlt}/></a>
}

const SearchModsForm = ({setIsFactorioAuthenticated, refetchInstalledMods, factorioVersion}) => {
    const {register, watch, handleSubmit} = useForm();
    const [results, setResults] = useState([]);
    const [isLoading, setIsLoading] = useState(false);
    const [fetchError, setFetchError] = useState(null);
    
    // For SelectVersionForm
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [releases, setReleases] = useState([]);
    const [selectedModName, setSelectedModName] = useState(null);

    const queryValue = watch('query') || "";

    const logout = () => {
        modsResource.portal.logout()
            .then(() => setIsFactorioAuthenticated(false));
    }

    useEffect(() => {
        const query = queryValue.trim();
        if (!query) {
            setResults([]);
            setFetchError(null);
            setIsLoading(false);
            return;
        }

        setIsLoading(true);
        setFetchError(null);

        const timer = setTimeout(() => {
            modsResource.portal.search(query, factorioVersion)
                .then(data => {
                    if (data && data.results) {
                        setResults(data.results);
                        setFetchError(null);
                    } else if (data && data.error) {
                        setResults([]);
                        setFetchError(data.error);
                    } else {
                        setResults([]);
                        setFetchError("No results found.");
                    }
                })
                .catch(() => {
                    setResults([]);
                    setFetchError("Failed to perform search.");
                })
                .finally(() => {
                    setIsLoading(false);
                });
        }, 400);

        return () => clearTimeout(timer);
    }, [queryValue, factorioVersion]);

    const install = async release => {
        if (!selectedModName) return;
        return modsResource.portal
            .install(release.download_url, release.file_name, selectedModName)
            .then(refetchInstalledMods);
    }

    const handleInstallClick = async (modName) => {
        try {
            const mod = await modsResource.portal.info(modName);
            setReleases(mod.releases || []);
            setSelectedModName(modName);
            setIsModalOpen(true);
        } catch (e) {
            console.error("Failed to fetch mod info for installation", e);
        }
    }

    return (
        <form onSubmit={e => e.preventDefault()}>
            <SelectVersionForm isOpen={isModalOpen} releases={releases} install={install} close={() => setIsModalOpen(false)}/>
            
            <div className="mb-4">
                <Label text="Search Query" htmlFor="query"/>
                <Input
                    register={register('query')}
                    placeholder="e.g. space exploration"
                    hasAutoComplete={false}
                />
            </div>

            {isLoading && (
                <div className="mb-4 py-2 px-3 text-white border border-gray-medium">
                    <FontAwesomeIcon icon={faSpinner} spin={true} className="mr-2"/> Searching mod portal <LinkModPortal/>
                </div>
            )}

            {fetchError && !isLoading && (
                <div className="mb-4 py-2 px-3 text-red border border-red">
                    {fetchError}
                </div>
            )}

            {!isLoading && results.length > 0 && (
                <div className="mb-4 flex flex-col gap-4">
                    {results.map((modInfo) => (
                        <div key={modInfo.name} className="p-4 border border-gray-medium bg-gray-dark text-white">
                            <div className="flex justify-between items-start mb-2">
                                <div>
                                    <h3 className="text-xl font-bold text-white">
                                        {modInfo.title} <span className="text-sm font-normal text-gray-light">({modInfo.name})</span>
                                    </h3>
                                    <div className="text-sm text-gray-light mt-1">
                                        By <span className="text-white font-semibold">{modInfo.owner || "Unknown"}</span>
                                        {modInfo.downloads_count !== undefined && (
                                            <span className="ml-4">
                                                <FontAwesomeIcon icon={faDownload} className="mr-1"/>
                                                {modInfo.downloads_count.toLocaleString()} downloads
                                            </span>
                                        )}
                                    </div>
                                </div>
                                <a
                                    href={`https://mods.factorio.com/mod/${modInfo.name}`}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="px-2 text-blue hover:text-blue-light text-sm"
                                >
                                    View on Mod Portal <FontAwesomeIcon icon={faExternalLinkAlt}/>
                                </a>
                            </div>
                            {modInfo.summary && (
                                <p className="text-gray-light text-sm mt-2 mb-3">
                                    {modInfo.summary}
                                </p>
                            )}
                            <div className="mt-3">
                                <Button type="button" onClick={() => handleInstallClick(modInfo.name)} className="mr-2">
                                    Install Mod
                                </Button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {!isLoading && queryValue.trim() && results.length === 0 && !fetchError && (
                <div className="mb-4 py-2 px-3 text-gray-light border border-gray-medium">
                    No results found for "{queryValue}".
                </div>
            )}

            <div className="flex items-center mt-4">
                <Button onClick={logout} type="danger" className="mr-2">
                    Logout
                </Button>
                <LinkModPortal/>
            </div>
        </form>
    )
}

export default SearchModsForm;
