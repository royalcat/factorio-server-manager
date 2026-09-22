import {coerce} from "semver";

// Factorio mod versions may contain zero-padded numeric segments (e.g. "2026.07.17").
// semver's coerce() rejects leading zeros, so strip them before parsing.
export const parseVersion = version => {
    if (version === null || version === undefined || version === "") {
        return null;
    }

    const normalized = String(version).trim().replace(/(^|\.)0+(?=\d)/g, "$1");
    return coerce(normalized);
};

export const formatFactorioVersion = version => {
    if (!version) {
        return "Unknown";
    }

    const parts = String(version).split(".");
    if (parts.length >= 3) {
        return parts.slice(0, 3).join(".");
    }

    return String(version);
};

export const formatFactorioVersionShort = version => {
    if (!version) {
        return "Unknown";
    }

    const parts = String(version).split(".");
    return parts.slice(0, 2).join(".");
};

export const formatModVersion = version => {
    if (!version) {
        return "Unknown";
    }

    const parts = String(version).split(".");
    return parts.slice(0, 3).join(".");
};
