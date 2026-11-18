const VERBATIM_PREFIX = "\\\\?\\";
const NT_PREFIX = "\\??\\";
const UNC_PREFIX = "\\\\?\\UNC\\";
const MAX_PATH = 248;

const isSep = (ch) => {
    return ch === "\\";
};

const startsWithVerbatim = (p) => {
    return p.startsWith(VERBATIM_PREFIX) || p.startsWith(NT_PREFIX);
};

const isLetter = (c) => {
    const code = c.charCodeAt(0);
    return (code >= 65 && code <= 90) || (code >= 97 && code <= 122);
};

const isDrivePrefix = (p) => {
    if (p.length < 2 || !isLetter(p[0]) || p[1] !== ":") {
        return false;
    }
    return p.length === 2 || isSep(p[2]);
};

const isUncPrefix = (p) => {
    return p.startsWith("\\\\");
};

const isDnPrefix = (p) => {
    return p.startsWith("\\\\.\\");
};

export const longPath = (path) => {
    if (path.length === 0 || startsWithVerbatim(path)) {
        return path;
    }

    if (path.length < MAX_PATH && (isDrivePrefix(path) || isUncPrefix(path))) {
        return path;
    }

    if (path.length + 1 >= MAX_PATH) {
        let prefix = "";
        if (isDrivePrefix(path)) {
            prefix = VERBATIM_PREFIX; // C:\ => \\?\C:\
        } else if (isDnPrefix(p)) {
            path = path.slice(4); // \\.\ => \\?\
            prefix = VERBATIM_PREFIX;
        } else if (isUncPrefix(path)) {
            path = path.slice(2); // \\ => \\?\UNC\
            prefix = UNC_PREFIX;
        }
        return prefix + path;
    }

    return path;
};
