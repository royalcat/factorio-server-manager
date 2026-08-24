import React, {useEffect, useState} from "react";
import modsResource from "../../../../../../api/resources/mods";
import Button from "../../../../../components/Button";
import Label from "../../../../../components/Label";
import {useForm} from "react-hook-form";
import Input from "../../../../../components/Input";
import {FontAwesomeIcon} from "@fortawesome/react-fontawesome";
import {faDownload, faExternalLinkAlt, faSpinner} from "@fortawesome/free-solid-svg-icons";
import SelectVersionForm from "./SelectVersionForm";

const LinkModPortal = () => {
    return <a href="https://mods.factorio.com" target="_blank" rel="noopener noreferrer" className="px-2 text-blue hover:text-blue-light">Mod
        Portal <FontAwesomeIcon icon={faExternalLinkAlt}/></a>
}

const extractSlug = input => {
    if (!input) return "";
    let str = input.trim();
    const match = str.match(/(?:https?:\/\/mods\.factorio\.com)?\/mod\/([^\/\?\#]+)/i);
    if (match && match[1]) {
        return match[1];
    }
    if (str.startsWith("http://") || str.startsWith("https://")) {
        try {
            const url = new URL(str);
            const pathParts = url.pathname.split("/").filter(Boolean);
            if (pathParts.length > 0) {
                return pathParts[pathParts.length - 1];
            }
        } catch (e) {
            // ignore
        }
    }
    return str.replace(/^\/+|\/+$/g, "");
};

const AddModForm = ({setIsFactorioAuthenticated, refetchInstalledMods}) => {

    const {register, watch, handleSubmit} = useForm();
    const [modInfo, setModInfo] = useState(null);
    const [releases, setReleases] = useState([]);
    const [isLoadingInfo, setIsLoadingInfo] = useState(false);
    const [fetchError, setFetchError] = useState(null);
    const [isModalOpen, setIsModalOpen] = useState(false);

    const modInputValue = watch('mod') || "";

    const logout = () => {
        modsResource.portal.logout()
            .then(() => setIsFactorioAuthenticated(false));
    }

    useEffect(() => {
        const slug = extractSlug(modInputValue);
        if (!slug) {
            setModInfo(null);
            setReleases([]);
            setFetchError(null);
            setIsLoadingInfo(false);
            return;
        }

        setIsLoadingInfo(true);
        setFetchError(null);

        const timer = setTimeout(() => {
            modsResource.portal.info(slug)
                .then(data => {
                    if (data && data.name) {
                        setModInfo(data);
                        setReleases(data.releases || []);
                        setFetchError(null);
                    } else if (data && data.error) {
                        setModInfo(null);
                        setReleases([]);
                        setFetchError(data.error);
                    } else {
                        setModInfo(null);
                        setReleases([]);
                        setFetchError("Mod not found on portal");
                    }
                })
                .catch(() => {
                    setModInfo(null);
                    setReleases([]);
                    setFetchError("Failed to fetch mod info");
                })
                .finally(() => {
                    setIsLoadingInfo(false);
                });
        }, 400);

        return () => clearTimeout(timer);
    }, [modInputValue]);

    const install = async release => {
        if (!modInfo) return;
        return modsResource.portal
            .install(release.download_url, release.file_name, modInfo.name)
            .then(refetchInstalledMods);
    }

    const onSubmit = () => {
        if (modInfo) {
            setIsModalOpen(true);
        }
    }

    return (
        <form onSubmit={handleSubmit(onSubmit)}>
            <SelectVersionForm isOpen={isModalOpen} releases={releases} install={install} close={() => setIsModalOpen(false)}/>
            
            <div className="mb-4">
                <Label text="Mod Slug or URL" htmlFor="mod"/>
                <Input
                    register={register('mod', {required: true})}
                    placeholder="e.g. helmod or https://mods.factorio.com/mod/helmod"
                    hasAutoComplete={false}
                />
            </div>

            {isLoadingInfo && (
                <div className="mb-4 py-2 px-3 text-white border border-gray-medium">
                    <FontAwesomeIcon icon={faSpinner} spin={true} className="mr-2"/> Loading mod info from <LinkModPortal/>
                </div>
            )}

            {fetchError && !isLoadingInfo && (
                <div className="mb-4 py-2 px-3 text-red border border-red">
                    {fetchError}
                </div>
            )}

            {modInfo && !isLoadingInfo && (
                <div className="mb-4 p-4 border border-gray-medium bg-gray-dark text-white">
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
                        <Button type="button" onClick={() => setIsModalOpen(true)} className="mr-2">
                            Install Mod
                        </Button>
                    </div>
                </div>
            )}

            <div className="flex items-center">
                <Button isDisabled={!modInfo || isLoadingInfo} isSubmit={true} className="mr-2">
                    Install
                </Button>
                <Button onClick={logout} type="danger" className="mr-2">
                    Logout
                </Button>
                <LinkModPortal/>
            </div>
        </form>
    )
}

export default AddModForm;
