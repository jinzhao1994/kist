// Generated from the Go HTTP contract. Run npm run generate:api.
export interface paths {
    "/api/v1/ai": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai */
        get: operations["get-api-v1-ai"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/accounts/{number}/connect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai accounts by number connect */
        post: operations["post-api-v1-ai-accounts-by-number-connect"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/accounts/{number}/disconnect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai accounts by number disconnect */
        post: operations["post-api-v1-ai-accounts-by-number-disconnect"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/accounts/{number}/models": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 ai accounts by number models */
        get: operations["list-api-v1-ai-accounts-by-number-models"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/connect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai connect */
        post: operations["post-api-v1-ai-connect"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/desc": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai desc */
        get: operations["get-api-v1-ai-desc"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/desc/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai desc backfill */
        post: operations["post-api-v1-ai-desc-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/desc/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai desc jobs */
        get: operations["get-api-v1-ai-desc-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/desc/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai desc pause */
        post: operations["post-api-v1-ai-desc-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/disconnect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai disconnect */
        post: operations["post-api-v1-ai-disconnect"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/facts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai facts */
        get: operations["get-api-v1-ai-facts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/facts/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai facts backfill */
        post: operations["post-api-v1-ai-facts-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/facts/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai facts jobs */
        get: operations["get-api-v1-ai-facts-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/facts/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai facts pause */
        post: operations["post-api-v1-ai-facts-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/models": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 ai models */
        get: operations["list-api-v1-ai-models"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/ocr": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai ocr */
        get: operations["get-api-v1-ai-ocr"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/ocr/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai ocr backfill */
        post: operations["post-api-v1-ai-ocr-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/ocr/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai ocr jobs */
        get: operations["get-api-v1-ai-ocr-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/ocr/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai ocr pause */
        post: operations["post-api-v1-ai-ocr-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/orientation": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai orientation */
        get: operations["get-api-v1-ai-orientation"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/orientation/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai orientation backfill */
        post: operations["post-api-v1-ai-orientation-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/orientation/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai orientation jobs */
        get: operations["get-api-v1-ai-orientation-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/orientation/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai orientation pause */
        post: operations["post-api-v1-ai-orientation-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/services": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai services */
        get: operations["get-api-v1-ai-services"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai settings */
        get: operations["get-api-v1-ai-settings"];
        /** Put API v1 ai settings */
        put: operations["put-api-v1-ai-settings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/transcription": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai transcription */
        get: operations["get-api-v1-ai-transcription"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/transcription/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai transcription backfill */
        post: operations["post-api-v1-ai-transcription-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/transcription/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ai transcription jobs */
        get: operations["get-api-v1-ai-transcription-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ai/transcription/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ai transcription pause */
        post: operations["post-api-v1-ai-transcription-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes */
        get: operations["get-api-v1-boxes"];
        put?: never;
        /** Post API v1 boxes */
        post: operations["post-api-v1-boxes"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/search": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes search */
        get: operations["get-api-v1-boxes-search"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{box}/entries/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 boxes by box entries by ID */
        delete: operations["delete-api-v1-boxes-by-box-entries-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{box}/entries/{id}/deletion-preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by box entries by ID deletion preview */
        get: operations["get-api-v1-boxes-by-box-entries-by-id-deletion-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{box}/entries/{id}/name": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 boxes by box entries by ID name */
        put: operations["put-api-v1-boxes-by-box-entries-by-id-name"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID */
        get: operations["get-api-v1-boxes-by-id"];
        /** Put API v1 boxes by ID */
        put: operations["put-api-v1-boxes-by-id"];
        post?: never;
        /** Delete API v1 boxes by ID */
        delete: operations["delete-api-v1-boxes-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/cover": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID cover */
        get: operations["get-api-v1-boxes-by-id-cover"];
        /** Put API v1 boxes by ID cover */
        put: operations["put-api-v1-boxes-by-id-cover"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/deletion-preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID deletion preview */
        get: operations["get-api-v1-boxes-by-id-deletion-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/folders": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID folders */
        get: operations["get-api-v1-boxes-by-id-folders"];
        put?: never;
        /** Post API v1 boxes by ID folders */
        post: operations["post-api-v1-boxes-by-id-folders"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 boxes by ID tags */
        get: operations["get-api-v1-boxes-by-id-tags"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/boxes/{id}/uploads": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Upload a complete batch with a JSON manifest followed by file parts
         * @description The first multipart field is manifest: {files:[{path,last_modified,size,sha256}]}, with modification times in Unix milliseconds, byte lengths and lowercase SHA-256 digests calculated from original client files. All fields are required; outdated clients must refresh. Remaining fields are files, with filenames matching manifest paths. Retries bind paths, timestamps and content hashes, not multipart boundaries.
         */
        post: operations["upload-files"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/catalog-revision": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 catalog revision */
        get: operations["get-api-v1-catalog-revision"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/entries": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 entries */
        get: operations["get-api-v1-entries"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files */
        get: operations["get-api-v1-files"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID */
        get: operations["get-api-v1-files-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read original file bytes; supports HTTP ranges */
        get: operations["GET-file-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        /** Read original file bytes; supports HTTP ranges */
        head: operations["HEAD-file-content"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/derived/{recipe}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID derived by recipe */
        get: operations["get-api-v1-files-by-id-derived-by-recipe"];
        put?: never;
        /** Post API v1 files by ID derived by recipe */
        post: operations["post-api-v1-files-by-id-derived-by-recipe"];
        /** Delete API v1 files by ID derived by recipe */
        delete: operations["delete-api-v1-files-by-id-derived-by-recipe"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/derived/{recipe}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read completed derived media; supports HTTP ranges */
        get: operations["GET-derived-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        /** Read completed derived media; supports HTTP ranges */
        head: operations["HEAD-derived-content"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/desc": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID desc */
        get: operations["get-api-v1-files-by-id-desc"];
        put?: never;
        /** Post API v1 files by ID desc */
        post: operations["post-api-v1-files-by-id-desc"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/desc/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID desc attempts */
        get: operations["list-api-v1-files-by-id-desc-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/desc/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID desc attempts by request ID */
        get: operations["get-api-v1-files-by-id-desc-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/extractions/{extractor}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 files by ID extractions by extractor */
        post: operations["post-api-v1-files-by-id-extractions-by-extractor"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/facts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID facts */
        get: operations["get-api-v1-files-by-id-facts"];
        put?: never;
        /** Post API v1 files by ID facts */
        post: operations["post-api-v1-files-by-id-facts"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/facts/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID facts attempts */
        get: operations["list-api-v1-files-by-id-facts-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/facts/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID facts attempts by request ID */
        get: operations["get-api-v1-files-by-id-facts-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/geo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID geo */
        get: operations["get-api-v1-files-by-id-geo"];
        put?: never;
        /** Post API v1 files by ID geo */
        post: operations["post-api-v1-files-by-id-geo"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/geo/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID geo attempts */
        get: operations["list-api-v1-files-by-id-geo-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/geo/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID geo attempts by request ID */
        get: operations["get-api-v1-files-by-id-geo-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/image-rotation": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 files by ID image rotation */
        put: operations["put-api-v1-files-by-id-image-rotation"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/metadata": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID metadata */
        get: operations["get-api-v1-files-by-id-metadata"];
        /** Put API v1 files by ID metadata */
        put: operations["put-api-v1-files-by-id-metadata"];
        /** Post API v1 files by ID metadata */
        post: operations["post-api-v1-files-by-id-metadata"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/metadata/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID metadata attempts */
        get: operations["list-api-v1-files-by-id-metadata-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/metadata/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID metadata attempts by request ID */
        get: operations["get-api-v1-files-by-id-metadata-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/ocr": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID ocr */
        get: operations["get-api-v1-files-by-id-ocr"];
        put?: never;
        /** Post API v1 files by ID ocr */
        post: operations["post-api-v1-files-by-id-ocr"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/ocr/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID ocr attempts */
        get: operations["list-api-v1-files-by-id-ocr-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/ocr/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID ocr attempts by request ID */
        get: operations["get-api-v1-files-by-id-ocr-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/orientation": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID orientation */
        get: operations["get-api-v1-files-by-id-orientation"];
        put?: never;
        /** Post API v1 files by ID orientation */
        post: operations["post-api-v1-files-by-id-orientation"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/orientation/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID orientation attempts */
        get: operations["list-api-v1-files-by-id-orientation-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/orientation/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID orientation attempts by request ID */
        get: operations["get-api-v1-files-by-id-orientation-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/playback/index.m3u8": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read the VOD timeline; segments are generated only when requested */
        get: operations["get-video-playlist"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/playback/segments/{segment}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Read or generate one bounded HLS segment */
        get: operations["GET-video-segment"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        /** Read or generate one bounded HLS segment */
        head: operations["HEAD-video-segment"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/sheet-preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID sheet preview */
        get: operations["get-api-v1-files-by-id-sheet-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/text-preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID text preview */
        get: operations["get-api-v1-files-by-id-text-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/transcription": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID transcription */
        get: operations["get-api-v1-files-by-id-transcription"];
        put?: never;
        /** Post API v1 files by ID transcription */
        post: operations["post-api-v1-files-by-id-transcription"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/transcription/attempts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 files by ID transcription attempts */
        get: operations["list-api-v1-files-by-id-transcription-attempts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/files/{id}/transcription/attempts/{request_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 files by ID transcription attempts by request ID */
        get: operations["get-api-v1-files-by-id-transcription-attempts-by-request-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/filesets": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 filesets */
        get: operations["get-api-v1-filesets"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/filesets/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 filesets by ID */
        get: operations["get-api-v1-filesets-by-id"];
        /** Put API v1 filesets by ID */
        put: operations["put-api-v1-filesets-by-id"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/geo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 geo */
        get: operations["get-api-v1-geo"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/geo/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 geo backfill */
        post: operations["post-api-v1-geo-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/geo/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 geo jobs */
        get: operations["get-api-v1-geo-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/geo/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 geo pause */
        post: operations["post-api-v1-geo-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/geo/settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 geo settings */
        get: operations["get-api-v1-geo-settings"];
        /** Put API v1 geo settings */
        put: operations["put-api-v1-geo-settings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/history/{op}/boxes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 history by op boxes by ID */
        get: operations["get-api-v1-history-by-op-boxes-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/history/{op}/files/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 history by op files by ID */
        get: operations["get-api-v1-history-by-op-files-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/info": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 info */
        get: operations["get-api-v1-info"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/map-provider": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 map provider */
        get: operations["get-api-v1-map-provider"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/metadata": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 metadata */
        get: operations["get-api-v1-metadata"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/metadata/backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 metadata backfill */
        post: operations["post-api-v1-metadata-backfill"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/metadata/jobs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 metadata jobs */
        get: operations["get-api-v1-metadata-jobs"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/metadata/pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 metadata pause */
        post: operations["post-api-v1-metadata-pause"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/navigation": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 navigation */
        get: operations["get-api-v1-navigation"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ops */
        get: operations["get-api-v1-ops"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ops by ID */
        get: operations["get-api-v1-ops-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops/{id}/summary": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 ops by ID summary */
        get: operations["get-api-v1-ops-by-id-summary"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/ops/{id}/undo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 ops by ID undo */
        post: operations["post-api-v1-ops-by-id-undo"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/requests/{key}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 requests by key */
        get: operations["get-api-v1-requests-by-key"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/saved-searches": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 saved searches */
        get: operations["get-api-v1-saved-searches"];
        put?: never;
        /** Post API v1 saved searches */
        post: operations["post-api-v1-saved-searches"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/saved-searches/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 saved searches by ID */
        put: operations["put-api-v1-saved-searches-by-id"];
        post?: never;
        /** Delete API v1 saved searches by ID */
        delete: operations["delete-api-v1-saved-searches-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 settings */
        get: operations["get-api-v1-settings"];
        /** Put API v1 settings */
        put: operations["put-api-v1-settings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/settings/credentials": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List API v1 settings credentials */
        get: operations["list-api-v1-settings-credentials"];
        put?: never;
        /** Post API v1 settings credentials */
        post: operations["post-api-v1-settings-credentials"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/settings/credentials/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 settings credentials by ID */
        put: operations["put-api-v1-settings-credentials-by-id"];
        post?: never;
        /** Delete API v1 settings credentials by ID */
        delete: operations["delete-api-v1-settings-credentials-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/settings/storage": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 settings storage */
        get: operations["get-api-v1-settings-storage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/settings/video-cache/clear": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 settings video cache clear */
        post: operations["post-api-v1-settings-video-cache-clear"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["sync-capabilities"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-catalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/derived/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["GET-sync-derived-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head: operations["HEAD-sync-derived-content"];
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/derived/describe": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-derived-describe"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/head": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["sync-head"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/original-uploads": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-original-upload-create"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/original-uploads/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["sync-original-upload-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/original-uploads/{id}/files/{index}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["sync-original-upload-chunk"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/original-uploads/{id}/finish": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-original-upload-finish"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/original-uploads/{id}/prefix": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-original-upload-prefix"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/originals/{key}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["sync-original-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/originals/{key}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["sync-original-content"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/submit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-submit"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/submit-conditional": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["sync-submit-conditional"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 tags */
        get: operations["get-api-v1-tags"];
        put?: never;
        /** Post API v1 tags */
        post: operations["post-api-v1-tags"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 tags by ID */
        delete: operations["delete-api-v1-tags-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/boxes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["PUT-tag-boxes"];
        post?: never;
        delete: operations["DELETE-tag-boxes"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/files": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["PUT-tag-files"];
        post?: never;
        delete: operations["DELETE-tag-files"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/name": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 tags by ID name */
        put: operations["put-api-v1-tags-by-id-name"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/tags/{id}/parent": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 tags by ID parent */
        put: operations["put-api-v1-tags-by-id-parent"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 upload sessions */
        post: operations["post-api-v1-upload-sessions"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 upload sessions by ID */
        get: operations["get-api-v1-upload-sessions-by-id"];
        put?: never;
        post?: never;
        /** Delete API v1 upload sessions by ID */
        delete: operations["delete-api-v1-upload-sessions-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions/{id}/complete": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 upload sessions by ID complete */
        post: operations["post-api-v1-upload-sessions-by-id-complete"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions/{id}/files/{index}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Append a verified sequential block, or retry an identical confirmed block */
        put: operations["upload-session-chunk"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions/{id}/prefix": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 upload sessions by ID prefix */
        post: operations["post-api-v1-upload-sessions-by-id-prefix"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/upload-sessions/{id}/resume": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 upload sessions by ID resume */
        post: operations["post-api-v1-upload-sessions-by-id-resume"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        AIAccountSettings: {
            api_key?: string;
            clear_secret?: boolean;
            readonly connected: boolean;
            readonly email: string;
            id: string;
            /** Format: int64 */
            number: number;
            /** Format: int64 */
            proxy: number;
            /** @enum {string} */
            type: "chatgpt" | "openai";
        };
        AIOptions: {
            auto_desc: boolean;
            auto_facts: boolean;
            auto_ocr: boolean;
            auto_orientation: boolean;
            model: string;
            proxy_url: string;
        };
        AIProxySettings: {
            clear_secret?: boolean;
            host: string;
            id: string;
            /** Format: int64 */
            number: number;
            password?: string;
            /** Format: int64 */
            port: number;
            readonly secret_configured: boolean;
            server_name: string;
            /** @enum {string} */
            type: "http" | "trojan";
            url: string;
        };
        AIServiceSettings: {
            accounts: components["schemas"]["AIAccountSettings"][] | null;
            features: {
                [key: string]: components["schemas"]["Feature"];
            };
            proxies: components["schemas"]["AIProxySettings"][] | null;
            revision: string;
        };
        AIStatus: {
            connected: boolean;
            email: string;
            error: string;
            pending: boolean;
            supported: boolean;
        };
        AiAccountActionBody: Record<string, never>;
        AiServicesView: {
            accounts: components["schemas"]["AIAccountSettings"][] | null;
            error: string;
            /** Format: int64 */
            error_account: number;
            features: {
                [key: string]: components["schemas"]["Feature"];
            };
            /** Format: int64 */
            pending_account: number;
            proxies: components["schemas"]["AIProxySettings"][] | null;
            revision: string;
        };
        AttachTagInputBody: {
            ids: string[] | null;
        };
        Box: {
            cover_file_id?: string;
            /** Format: date-time */
            created_at: string;
            description?: string;
            end_at?: string;
            id: string;
            name: string;
            start_at?: string;
            /** Format: int64 */
            total_size: number;
            /** Format: date-time */
            updated_at: string;
        };
        BoxBody: {
            description?: string;
            end_at?: string;
            name: string;
            start_at?: string;
        };
        BoxCover: {
            file?: components["schemas"]["File"];
        };
        BoxDeletionPreview: {
            /** Format: int64 */
            file_count: number;
            name: string;
            /** Format: int64 */
            revision: number;
        };
        BoxList: {
            boxes: components["schemas"]["Box"][] | null;
        };
        BoxSearchItem: {
            cover_file_id?: string;
            /** Format: date-time */
            created_at: string;
            description?: string;
            end_at?: string;
            id: string;
            name: string;
            start_at?: string;
            tags?: components["schemas"]["DisplayTag"][] | null;
            /** Format: int64 */
            total_size: number;
            /** Format: date-time */
            updated_at: string;
        };
        BoxSearchResult: {
            boxes: components["schemas"]["BoxSearchItem"][] | null;
            cursor?: string;
        };
        CatalogRevision: {
            revision: string;
        };
        Cell: {
            missing_result?: boolean;
            text: string;
        };
        ClientConnection: {
            allow_http: boolean;
            server: string;
        };
        CreateTagInputBody: {
            name: string;
            parent_id?: string;
        };
        "Delete-api-v1-boxes-by-idRequest": {
            /** Format: int64 */
            revision: number;
        };
        DisplayTag: {
            id: string;
            name: string;
        };
        EntryDeleteInputBody: {
            /** Format: int64 */
            revision: number;
        };
        EntryDeletionPreview: {
            /** Format: int64 */
            file_count: number;
            /** Format: int64 */
            folder_count: number;
            name: string;
            /** Format: int64 */
            revision: number;
        };
        EntryRenameInputBody: {
            before: string;
            name: string;
        };
        Error: {
            /** Format: int64 */
            blocking_op?: number;
            candidates?: components["schemas"]["Ref"][] | null;
            class: string;
            code: string;
            existing_id?: string;
            holder?: string;
            message: string;
            /** Format: int64 */
            op_id?: number;
            /** Format: int64 */
            reverted_by?: number;
            /** Format: int64 */
            upload_offset?: number;
        };
        ExtractionAttempt: {
            blob_key: string;
            error?: string;
            file_id: string;
            finished_at: string;
            kind: string;
            model?: string;
            /** Format: int64 */
            output_bytes: number;
            provider?: string;
            request_id: string;
            /** Format: int64 */
            sequence: number;
            started_at: string;
            state: string;
            truncated: boolean;
        };
        ExtractionAttemptDetail: {
            blob_key: string;
            error?: string;
            file_id: string;
            finished_at: string;
            kind: string;
            model?: string;
            /** Format: int64 */
            output_bytes: number;
            output_text: string;
            provider?: string;
            request_id: string;
            /** Format: int64 */
            sequence: number;
            started_at: string;
            state: string;
            truncated: boolean;
        };
        ExtractionInputBody: {
            force?: boolean;
        };
        ExtractionQueueItem: {
            box?: components["schemas"]["QueueItemBox"];
            error?: string;
            file_available: boolean;
            file_id: string;
            name: string;
            request_id: string;
            /** @enum {string} */
            state: "queued" | "failed";
        };
        ExtractionQueuePage: {
            items: components["schemas"]["ExtractionQueueItem"][] | null;
            next_cursor: string;
        };
        Feature: {
            /** Format: int64 */
            account: number;
            automatic: boolean;
            image_detail: string;
            model: string;
            reasoning_effort: string;
        };
        Field: {
            default: unknown;
            explicit: boolean;
            key: string;
            kind: string;
            options?: string[] | null;
            restart: boolean;
            unlimited?: boolean;
            value: unknown;
        };
        File: {
            blob_key: string;
            box_id?: string;
            /** Format: date-time */
            captured_at: string | null;
            /** Format: date-time */
            created_at: string;
            extraction_pending?: string[] | null;
            folder_id: string;
            id: string;
            /**
             * Format: int64
             * @enum {integer}
             */
            image_rotation?: 0 | 90 | 180 | 270;
            image_rotation_manual?: boolean;
            kind?: string;
            media_type?: string;
            name: string;
            path?: string;
            /** Format: int64 */
            size?: number;
            type_label?: string;
            /** Format: date-time */
            updated_at: string;
        };
        FileDetail: {
            captured_from?: string;
            captured_zone?: string;
            extracted: string[] | null;
            file: components["schemas"]["File"];
            gps?: components["schemas"]["Point"];
            metadata: components["schemas"]["Metadata"][] | null;
            tags: string[] | null;
        };
        FileSearchItem: {
            blob_key: string;
            box_id?: string;
            /** Format: date-time */
            captured_at: string | null;
            /** Format: date-time */
            created_at: string;
            display_tags?: components["schemas"]["DisplayTag"][] | null;
            extraction_pending?: string[] | null;
            fileset_id?: string;
            folder_id: string;
            id: string;
            /**
             * Format: int64
             * @enum {integer}
             */
            image_rotation?: 0 | 90 | 180 | 270;
            image_rotation_manual?: boolean;
            kind?: string;
            media_type?: string;
            name: string;
            no_primary?: boolean;
            path?: string;
            primary_id?: string;
            primary_name?: string;
            /** Format: int64 */
            size?: number;
            type_label?: string;
            /** Format: date-time */
            updated_at: string;
        };
        FileSearchResult: {
            cursor?: string;
            files: components["schemas"]["FileSearchItem"][] | null;
        };
        Fileset: {
            folder_id: string;
            kind: string;
            layout: string;
            members: components["schemas"]["FilesetMember"][] | null;
            primary: string;
        };
        FilesetDetail: {
            box_id: string;
            fileset: components["schemas"]["Fileset"];
            folder: components["schemas"]["Folder"];
        };
        FilesetList: {
            filesets: components["schemas"]["Fileset"][] | null;
        };
        FilesetMember: {
            default: boolean;
            file_id: string;
            folder_id: string;
            language: string;
            role: string;
        };
        FilesetWriteInputBody: {
            after?: components["schemas"]["Fileset"];
            before?: components["schemas"]["Fileset"];
            box: string;
        };
        Folder: {
            /** Format: date-time */
            created_at: string;
            id: string;
            name: string;
            parent_id: string | null;
            /** Format: int64 */
            total_size: number;
            /** Format: date-time */
            updated_at: string;
        };
        FolderList: {
            folders: components["schemas"]["Folder"][] | null;
        };
        HTTPError: {
            /** Format: int64 */
            blocking_op?: number;
            candidates?: components["schemas"]["Ref"][] | null;
            class: string;
            code: string;
            existing_id?: string;
            fields?: components["schemas"]["SettingFieldError"][] | null;
            holder?: string;
            message: string;
            /** Format: int64 */
            op_id?: number;
            /** Format: int64 */
            reverted_by?: number;
            /** Format: int64 */
            upload_offset?: number;
        };
        HistoricalBox: {
            box: components["schemas"]["Box"];
            box_tags: string[] | null;
            cursor?: string;
            files: components["schemas"]["File"][] | null;
            folders: components["schemas"]["Folder"][] | null;
            /** Format: int64 */
            op: number;
            tags: components["schemas"]["Tag"][] | null;
        };
        HistoricalFile: {
            file: components["schemas"]["FileDetail"];
            /** Format: int64 */
            op: number;
            tags: components["schemas"]["Tag"][] | null;
        };
        HistoryDetail: {
            /** Format: date-time */
            accepted_at?: string;
            action: string;
            boxes: string[] | null;
            files: string[] | null;
            /** Format: int64 */
            id: number;
            origin: string;
            payload: {
                [key: string]: unknown;
            };
            /** Format: int64 */
            reverts_id: number | null;
            /** Format: int64 */
            root_op_id?: number;
            /** Format: date-time */
            ts: string;
            undoable: boolean;
            /** Format: int64 */
            undone_by: number | null;
        };
        HistoryList: {
            /** Format: int64 */
            before?: number;
            ops: components["schemas"]["HistoryOp"][] | null;
        };
        HistoryOp: {
            /** Format: date-time */
            accepted_at?: string;
            action: string;
            /** Format: int64 */
            id: number;
            origin: string;
            /** Format: int64 */
            reverts_id: number | null;
            /** Format: int64 */
            root_op_id?: number;
            /** Format: date-time */
            ts: string;
            undoable: boolean;
            /** Format: int64 */
            undone_by: number | null;
        };
        HistorySummary: {
            after?: string;
            before?: string;
            context?: string;
            /** Format: int64 */
            count: number;
            current_names: boolean;
            name: string;
            tag?: string;
        };
        LibraryInfo: {
            client?: boolean;
            /** Format: int64 */
            max_upload_bytes: number;
            /** Format: int64 */
            max_upload_files: number;
            primary_color: string;
            projection_dirty: boolean;
            /** Format: int64 */
            resumable_upload_version?: number;
            timezone: string;
            /** Format: int64 */
            upload_chunk_bytes?: number;
            version: string;
        };
        LocationSettingDefaults: {
            automatic: boolean;
            endpoint: string;
            pause_on_error: boolean;
            /** Format: int64 */
            request_interval_seconds: number;
        };
        LocationSettings: {
            automatic: boolean;
            changed?: string[] | null;
            defaults?: components["schemas"]["LocationSettingDefaults"];
            endpoint: string;
            readonly explicit: string[] | null;
            pause_on_error: boolean;
            /** Format: int64 */
            request_interval_seconds: number;
            reset?: string[] | null;
            revision: string;
        };
        MapProvider: {
            resource_origins: string[] | null;
            style_url: string;
            terrain_attribution: string;
            terrain_tiles: string;
        };
        MediaJob: {
            blob_key: string;
            generation: string;
            reason?: string;
            recipe: string;
            result: components["schemas"]["Result"];
            /** @enum {string} */
            state: "missing" | "queued" | "running" | "ready" | "blocked" | "failed" | "cancelled";
            /** Format: date-time */
            updated_at: string;
        };
        MediaRequestBody: {
            retry?: boolean;
        };
        Merge: {
            /** Format: int64 */
            column: number;
            /** Format: int64 */
            columns: number;
            /** Format: int64 */
            row: number;
            /** Format: int64 */
            rows: number;
        };
        Metadata: {
            key: string;
            source: string;
            type: string;
            value: string;
        };
        MetadataEdit: {
            before_source: string;
            before_type: string;
            before_value: string;
            blob_key: string;
            key: string;
            path?: string;
            remove?: boolean;
            value: string;
        };
        Model: {
            id: string;
            name: string;
        };
        MoveTagInputBody: {
            parent_id: string | null;
        };
        NavigationLocation: {
            box: string;
            file?: string;
            folder?: string;
            kind: string;
            path: string;
            view?: string;
        };
        NewSyncCredential: {
            /** Format: date-time */
            created_at: string;
            id: string;
            note?: string;
            token: string;
        };
        OCRJob: {
            automatic: boolean;
            blob_key: string;
            error?: string;
            extractor?: string;
            file_id: string;
            force?: boolean;
            manual?: boolean;
            name: string;
            request_id: string;
            /** @enum {string} */
            state: "missing" | "queued" | "running" | "ready" | "failed";
        };
        OCRQueueStatus: {
            error?: string;
            /** Format: int64 */
            failed: number;
            paused: boolean;
            /** Format: int64 */
            queued: number;
            running: string;
            scanning: boolean;
        };
        OCRSettings: {
            auto_desc: boolean;
            auto_facts: boolean;
            auto_ocr: boolean;
            auto_orientation: boolean;
            changed?: string[] | null;
            defaults?: components["schemas"]["AIOptions"];
            readonly explicit: string[] | null;
            model: string;
            proxy_url: string;
            reset?: string[] | null;
            revision: string;
        };
        "OutputStruct { Revoked bool \"json:\\\"revoked\\\"\" }Body": {
            revoked: boolean;
        };
        "OutputStruct { URL string \"json:\\\"url\\\"\" }Body": {
            url: string;
        };
        "OutputStruct {}Body": Record<string, never>;
        Point: {
            /** Format: double */
            accuracy?: number;
            /** Format: double */
            latitude: number;
            /** Format: double */
            longitude: number;
        };
        "Post-api-v1-ai-connectRequest": Record<string, never>;
        "Post-api-v1-ai-desc-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-ai-desc-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-ai-disconnectRequest": Record<string, never>;
        "Post-api-v1-ai-facts-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-ai-facts-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-ai-ocr-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-ai-ocr-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-ai-orientation-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-ai-orientation-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-ai-transcription-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-ai-transcription-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-boxes-by-id-foldersRequest": {
            name: string;
            parent?: string;
        };
        "Post-api-v1-files-by-id-descRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-factsRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-geoRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-metadataRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-ocrRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-orientationRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-files-by-id-transcriptionRequest": {
            /** @description Local metadata refresh only: optionally limit force=true to one registered extractor. */
            extractor?: string;
            force?: boolean;
            request_id?: string;
            retry: boolean;
        };
        "Post-api-v1-geo-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-geo-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-metadata-backfillRequest": {
            retry: boolean;
        };
        "Post-api-v1-metadata-pauseRequest": {
            paused: boolean;
        };
        "Post-api-v1-settings-credentialsRequest": {
            note?: string;
        };
        Preview: {
            encoding: string;
            reason?: string;
            supported: boolean;
            text: string;
            truncated: boolean;
        };
        "Put-api-v1-boxes-by-id-coverRequest": {
            file_id: string;
        };
        "Put-api-v1-files-by-id-image-rotationRequest": {
            blob_key: string;
            /** Format: int64 */
            clockwise_degrees: number | null;
        };
        "Put-api-v1-settings-credentials-by-idRequest": {
            note: string;
        };
        QueueItemBox: {
            name: string;
            start_at: string;
        };
        Ref: {
            id: string;
            name: string;
        };
        RenameTagInputBody: {
            name: string;
        };
        RequestResult: {
            /** Format: int64 */
            op_id: number;
        };
        Result: {
            /** Format: int64 */
            bytes: number;
            engine: string;
            /** Format: int64 */
            generation_ms: number;
            /** Format: int64 */
            height: number;
            /** Format: int64 */
            width: number;
        };
        SavedSearchBody: {
            definition: components["schemas"]["SearchDefinition"];
            name: string;
        };
        SavedSearchList: {
            searches: components["schemas"]["SavedSearchView"][] | null;
        };
        SavedSearchView: {
            definition: components["schemas"]["SearchDefinition"];
            id: string;
            name: string;
        };
        SearchCondition: {
            field: string;
            values: string[] | null;
        };
        SearchDefinition: {
            box_id?: string;
            entity: string;
            filters: components["schemas"]["SearchFilters"];
            folder_id?: string;
            /** Format: int64 */
            version: number;
        };
        SearchFilters: {
            asc: boolean;
            box?: string[] | null;
            conditions?: components["schemas"]["SearchCondition"][] | null;
            from?: string;
            include_members?: boolean;
            keyword?: string[] | null;
            kind?: string[] | null;
            name?: string;
            name_contains?: string;
            name_equals?: string;
            sort: string;
            tag_all?: string[] | null;
            tag_any?: string[] | null;
            tag_not?: string[] | null;
            to?: string;
        };
        SessionCommandBody: Record<string, never>;
        SessionCreateBody: {
            box: string;
            conflict_mode?: string;
            conflict_token?: string;
            files: components["schemas"]["UploadFile"][] | null;
            folder: string;
            /** Format: int64 */
            version: number;
        };
        SessionFinishBody: {
            digests?: string[] | null;
        };
        SessionPrefixBody: {
            /** Format: int64 */
            index: number;
            /** Format: int64 */
            offset: number;
            sha256: string;
        };
        SessionStatus: {
            conflicts?: components["schemas"]["UploadConflicts"];
            local_id?: string;
            noop?: boolean;
            /** Format: int64 */
            op_id?: number;
            session?: components["schemas"]["UploadState"];
        };
        SettingFieldError: {
            code: string;
            field: string;
            message: string;
        };
        SettingsInputBody: {
            ai?: components["schemas"]["OCRSettings"];
            ai_services?: components["schemas"]["AIServiceSettings"];
            changes: {
                [key: string]: unknown;
            };
            location?: components["schemas"]["LocationSettings"];
            revision: string;
        };
        SettingsResponse: {
            connection?: components["schemas"]["ClientConnection"];
            credential_configured: boolean;
            effective: {
                [key: string]: unknown;
            };
            fields: components["schemas"]["Field"][] | null;
            id: string;
            library_id: string;
            mode: string;
            overrides: string[] | null;
            restart_required: string[] | null;
            revision: string;
        };
        SheetPreview: {
            cells: (components["schemas"]["Cell"][] | null)[] | null;
            /** Format: int64 */
            column: number;
            encoding?: string;
            legacy?: boolean;
            merges: components["schemas"]["Merge"][] | null;
            more_columns: boolean;
            more_rows: boolean;
            reason?: string;
            /** Format: int64 */
            row: number;
            /** Format: int64 */
            sheet: number;
            sheets: string[] | null;
            supported: boolean;
            truncated: boolean;
            widths: number[] | null;
        };
        StorageUsage: {
            /** Format: int64 */
            metadata_bytes: number;
            /** Format: int64 */
            originals_bytes: number;
            /** Format: int64 */
            previews_bytes: number;
            /** Format: int64 */
            video_bytes: number;
        };
        "Sync-derived-describeRequest": {
            key: string;
            recipe: string;
            retry?: boolean;
        };
        "Sync-original-upload-createRequest": {
            key: string;
            /** Format: int64 */
            size: number;
        };
        "Sync-submit-conditionalRequest": {
            expected: components["schemas"]["SyncBoundary"];
            proposal: components["schemas"]["SyncProposal"];
        };
        SyncBoundary: {
            library_id: string;
            /** Format: int64 */
            op_id: number;
            sum: string;
        };
        SyncCapabilities: {
            actions: string[] | null;
            ancestors?: string[] | null;
            conditional_submit: boolean;
            deletion_snapshots: boolean;
            derived_previews: boolean;
            /** Format: int64 */
            journal_version: number;
            library_id: string;
            node_id?: string;
            relay?: boolean;
            timezone: string;
            undo_references: boolean;
            /** Format: int64 */
            version: number;
        };
        SyncCatalogInputBody: {
            /** Format: int64 */
            after: number;
            boundary: components["schemas"]["SyncBoundary"];
            /** Format: int64 */
            limit: number;
            /** Format: int64 */
            max_bytes: number;
        };
        SyncCatalogPage: {
            /** Format: int64 */
            after: number;
            boundary: components["schemas"]["SyncBoundary"];
            done: boolean;
            lines: unknown[] | null;
            /** Format: int64 */
            next: number;
        };
        SyncCredential: {
            /** Format: date-time */
            created_at: string;
            id: string;
            note?: string;
        };
        SyncDeletionBasis: {
            box: string;
            last: string;
        };
        SyncDerived: {
            content_key?: string;
            job: components["schemas"]["MediaJob"];
        };
        SyncFileState: {
            blob_key: string;
            id: string;
        };
        SyncOriginalStatus: {
            present: boolean;
            /** Format: int64 */
            size: number;
        };
        SyncOriginalUpload: {
            /** Format: int64 */
            chunk_bytes: number;
            id: string;
            key: string;
            /** Format: int64 */
            offset: number;
            /** Format: int64 */
            size: number;
        };
        SyncProposal: {
            action?: string;
            deletion?: components["schemas"]["SyncDeletionBasis"];
            files?: components["schemas"]["SyncFileState"][] | null;
            group?: components["schemas"]["SyncReference"];
            id: string;
            /** Format: date-time */
            occurred_at: string;
            origin: string;
            payload?: unknown;
            requires?: string[] | null;
            undo?: components["schemas"]["SyncReference"];
            /** Format: int64 */
            version: number;
        };
        SyncReference: {
            local_id?: string;
            /** Format: int64 */
            op_id?: number;
        };
        SyncResult: {
            /** Format: date-time */
            accepted_at: string;
            /** Format: int64 */
            op_id: number;
        };
        SyncSubmitResponse: {
            error?: components["schemas"]["Error"];
            rejected: boolean;
            result?: components["schemas"]["SyncResult"];
        };
        Tag: {
            /** Format: int64 */
            box_count?: number;
            children?: components["schemas"]["Tag"][] | null;
            /** Format: int64 */
            count: number | null;
            /** Format: int64 */
            file_count?: number;
            id: string;
            name: string;
            parent_id: string | null;
        };
        TagList: {
            tags: string[] | null;
        };
        TagTree: {
            tags: components["schemas"]["Tag"][] | null;
        };
        UndoInputBody: Record<string, never>;
        UpdateSavedSearchBody: {
            before: components["schemas"]["SavedSearchBody"];
            definition: components["schemas"]["SearchDefinition"];
            name: string;
        };
        UploadConflicts: {
            can_overwrite: boolean;
            paths: string[] | null;
            token: string;
        };
        UploadFile: {
            /** Format: int64 */
            last_modified: number;
            path: string;
            sha256: string;
            /** Format: int64 */
            size: number;
        };
        UploadManifest: {
            box: string;
            conflict_mode?: string;
            conflict_token?: string;
            destination_token?: string;
            files: components["schemas"]["UploadFile"][] | null;
            folder: string;
            /** Format: int64 */
            protocol?: number;
            target: string;
        };
        UploadState: {
            canceled?: boolean;
            digests?: string[] | null;
            /** Format: date-time */
            expires: string;
            id: string;
            local_id?: string;
            manifest: components["schemas"]["UploadManifest"];
            noop?: boolean;
            offsets: number[] | null;
            /** Format: int64 */
            op_id?: number;
            /** Format: int64 */
            version: number;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    "get-api-v1-ai": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AIStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-accounts-by-number-connect": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                number: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AiAccountActionBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { URL string \"json:\\\"url\\\"\" }Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-accounts-by-number-disconnect": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                number: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AiAccountActionBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { Revoked bool \"json:\\\"revoked\\\"\" }Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-ai-accounts-by-number-models": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                number: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Model"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-connect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-connectRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { URL string \"json:\\\"url\\\"\" }Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-desc": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-desc-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-desc-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-desc-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-desc-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-desc-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-disconnect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-disconnectRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { Revoked bool \"json:\\\"revoked\\\"\" }Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-facts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-facts-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-facts-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-facts-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-facts-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-facts-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-ai-models": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Model"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-ocr": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-ocr-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-ocr-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-ocr-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-ocr-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-ocr-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-orientation": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-orientation-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-orientation-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-orientation-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-orientation-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-orientation-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-services": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AiServicesView"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRSettings"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-ai-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["OCRSettings"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRSettings"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-transcription": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-transcription-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-transcription-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ai-transcription-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ai-transcription-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-ai-transcription-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes": {
        parameters: {
            query?: {
                tag?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BoxBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-search": {
        parameters: {
            query?: {
                condition?: string[] | null;
                keyword?: string[] | null;
                name_contains?: string;
                name_equals?: string;
                text?: string;
                tag_all?: string[] | null;
                tag_any?: string[] | null;
                tag_not?: string[] | null;
                from?: string;
                to?: string;
                sort?: "updated" | "name" | "activity";
                asc?: boolean;
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxSearchResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-boxes-by-box-entries-by-id": {
        parameters: {
            query?: {
                folder?: boolean;
            };
            header: {
                "Idempotency-Key": string;
            };
            path: {
                box: string;
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EntryDeleteInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-box-entries-by-id-deletion-preview": {
        parameters: {
            query?: {
                folder?: boolean;
            };
            header?: never;
            path: {
                box: string;
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EntryDeletionPreview"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-boxes-by-box-entries-by-id-name": {
        parameters: {
            query?: {
                folder?: boolean;
            };
            header: {
                "Idempotency-Key": string;
            };
            path: {
                box: string;
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EntryRenameInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Box"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BoxBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-boxes-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Delete-api-v1-boxes-by-idRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-cover": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxCover"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-boxes-by-id-cover": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Put-api-v1-boxes-by-id-coverRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-deletion-preview": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BoxDeletionPreview"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-folders": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FolderList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-boxes-by-id-folders": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-boxes-by-id-foldersRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-boxes-by-id-tags": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TagList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "upload-files": {
        parameters: {
            query?: {
                folder?: string;
            };
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": string;
            };
        };
        responses: {
            /** @description Committed request receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-catalog-revision": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogRevision"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-entries": {
        parameters: {
            query?: {
                condition?: string[] | null;
                keyword?: string[] | null;
                name_contains?: string;
                name_equals?: string;
                kind?: string[] | null;
                box?: string[] | null;
                folder?: string;
                /** @description Include descendants of folder */
                recursive?: boolean;
                tag_all?: string[] | null;
                tag_any?: string[] | null;
                tag_not?: string[] | null;
                name?: string;
                from?: string;
                to?: string;
                sort?: "captured" | "imported" | "name" | "updated" | "type" | "size";
                asc?: boolean;
                cursor?: string;
                limit?: number;
                include_members?: boolean;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FileSearchResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files": {
        parameters: {
            query?: {
                condition?: string[] | null;
                keyword?: string[] | null;
                name_contains?: string;
                name_equals?: string;
                kind?: string[] | null;
                box?: string[] | null;
                folder?: string;
                /** @description Include descendants of folder */
                recursive?: boolean;
                tag_all?: string[] | null;
                tag_any?: string[] | null;
                tag_not?: string[] | null;
                name?: string;
                from?: string;
                to?: string;
                sort?: "captured" | "imported" | "name" | "updated" | "type" | "size";
                asc?: boolean;
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FileSearchResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FileDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-file-content": {
        parameters: {
            query?: {
                download?: boolean;
                at?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Original file bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-file-content": {
        parameters: {
            query?: {
                download?: boolean;
                at?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Original file bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/octet-stream": string;
                };
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "get-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MediaRequestBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-files-by-id-derived-by-recipe": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MediaJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-derived-content": {
        parameters: {
            query?: {
                /** @description Expected media generation; matching versions can be cached privately for one year */
                v?: string;
            };
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Derived image, PDF, video or geographic data */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/geo+json": Record<string, never>;
                    "application/pdf": string;
                    "image/jpeg": string;
                    "image/png": string;
                    "video/mp4": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Requested generation is no longer current */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-derived-content": {
        parameters: {
            query?: {
                /** @description Expected media generation; matching versions can be cached privately for one year */
                v?: string;
            };
            header?: never;
            path: {
                id: string;
                recipe: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Derived image, PDF, video or geographic data */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/geo+json": Record<string, never>;
                    "application/pdf": string;
                    "image/jpeg": string;
                    "image/png": string;
                    "video/mp4": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Requested generation is no longer current */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "get-api-v1-files-by-id-desc": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-desc": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-descRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-desc-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-desc-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-extractions-by-extractor": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
                extractor: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ExtractionInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-facts": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-facts": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-factsRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-facts-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-facts-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-geo": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-geo": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-geoRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-geo-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-geo-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-files-by-id-image-rotation": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Put-api-v1-files-by-id-image-rotationRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-metadata": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-files-by-id-metadata": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MetadataEdit"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-metadata": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-metadataRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-metadata-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-metadata-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-ocr": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-ocr": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-ocrRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-ocr-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-ocr-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-orientation": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-orientation": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-orientationRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-orientation-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-orientation-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-video-playlist": {
        parameters: {
            query: {
                v: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description HLS VOD playlist */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/vnd.apple.mpegurl": string;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-video-segment": {
        parameters: {
            query: {
                v: string;
            };
            header?: never;
            path: {
                id: string;
                segment: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Independently decodable H.264/AAC segment */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "video/mp2t": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Video content changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-video-segment": {
        parameters: {
            query: {
                v: string;
            };
            header?: never;
            path: {
                id: string;
                segment: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Independently decodable H.264/AAC segment */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "video/mp2t": string;
                };
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Video content changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "get-api-v1-files-by-id-sheet-preview": {
        parameters: {
            query?: {
                /** @description Content version: sheet-v1:blob-key:filename */
                v?: string;
                detail?: boolean;
                sheet?: number;
                row?: number;
                column?: number;
                encoding?: "auto" | "gb18030";
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    "Cache-Control"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SheetPreview"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-text-preview": {
        parameters: {
            query?: {
                /** @description Read up to 1 MiB instead of an 8 KiB, 20-line thumbnail */
                detail?: boolean;
                encoding?: "auto" | "gb18030";
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Preview"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-transcription": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-files-by-id-transcription": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-files-by-id-transcriptionRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRJob"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-files-by-id-transcription-attempts": {
        parameters: {
            query?: {
                before?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttempt"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-files-by-id-transcription-attempts-by-request-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                request_id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionAttemptDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-filesets": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FilesetList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-filesets-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["FilesetDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-filesets-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["FilesetWriteInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-geo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-geo-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-geo-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-geo-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-geo-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-geo-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-geo-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LocationSettings"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-geo-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["LocationSettings"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LocationSettings"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-history-by-op-boxes-by-id": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path: {
                op: number;
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoricalBox"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-history-by-op-files-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                op: number;
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoricalFile"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-info": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LibraryInfo"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-map-provider": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MapProvider"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-metadata": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-metadata-backfill": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-metadata-backfillRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-metadata-jobs": {
        parameters: {
            query?: {
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ExtractionQueuePage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-metadata-pause": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-metadata-pauseRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OCRQueueStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-navigation": {
        parameters: {
            query?: {
                /** @description Escaped readable pathname, decoded once per component */
                path?: string;
                box?: string;
                folder?: string;
                /** @description File or fileset identity */
                file?: string;
                view?: "folder";
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["NavigationLocation"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ops": {
        parameters: {
            query?: {
                fileset?: string;
                box?: string;
                file?: string;
                action?: string;
                group?: number;
                before?: number;
                limit?: number;
                trash?: boolean;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoryList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ops-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistoryDetail"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-ops-by-id-summary": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HistorySummary"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-ops-by-id-undo": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UndoInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-requests-by-key": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                key: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-saved-searches": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SavedSearchList"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-saved-searches": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SavedSearchBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-saved-searches-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["UpdateSavedSearchBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-saved-searches-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SettingsResponse"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SettingsInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SettingsResponse"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "list-api-v1-settings-credentials": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncCredential"][] | null;
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-settings-credentials": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-settings-credentialsRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["NewSyncCredential"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-settings-credentials-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Put-api-v1-settings-credentials-by-idRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct {}Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-settings-credentials-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct {}Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-settings-storage": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["StorageUsage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-settings-video-cache-clear": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct {}Body"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncCapabilities"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SyncCatalogInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncCatalogPage"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "GET-sync-derived-content": {
        parameters: {
            query: {
                key: string;
                recipe: string;
                generation: string;
                content_key: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Immutable derived bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description The requested generation changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "HEAD-sync-derived-content": {
        parameters: {
            query: {
                key: string;
                recipe: string;
                generation: string;
                content_key: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Immutable derived bytes */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Requested byte range */
            206: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description The requested generation changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unsatisfiable byte range */
            416: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    "sync-derived-describe": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Sync-derived-describeRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncDerived"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-head": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncBoundary"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-upload-create": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Sync-original-upload-createRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalUpload"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-upload-status": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalUpload"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-upload-chunk": {
        parameters: {
            query: {
                offset: number;
            };
            header: {
                "Upload-SHA256": string;
                "Upload-Length": number;
            };
            path: {
                id: string;
                index: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/octet-stream": string;
            };
        };
        responses: {
            /** @description Durable original upload progress */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalUpload"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-upload-finish": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalUpload"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-upload-prefix": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SessionPrefixBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalUpload"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-status": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                key: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncOriginalStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-original-content": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                key: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-submit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SyncProposal"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncSubmitResponse"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "sync-submit-conditional": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Sync-submit-conditionalRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SyncSubmitResponse"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-tags": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TagTree"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-tags": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-tags-by-id": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "PUT-tag-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "DELETE-tag-boxes": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "PUT-tag-files": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "DELETE-tag-files": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AttachTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-tags-by-id-name": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RenameTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "put-api-v1-tags-by-id-parent": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MoveTagInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RequestResult"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-upload-sessions": {
        parameters: {
            query?: never;
            header: {
                "Idempotency-Key": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SessionCreateBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "get-api-v1-upload-sessions-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "delete-api-v1-upload-sessions-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-upload-sessions-by-id-complete": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SessionFinishBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "upload-session-chunk": {
        parameters: {
            query: {
                offset: number;
            };
            header: {
                "Upload-SHA256": string;
                "Upload-Length": number;
            };
            path: {
                id: string;
                index: number;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/octet-stream": string;
            };
        };
        responses: {
            /** @description Durable progress or archive receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-upload-sessions-by-id-prefix": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SessionPrefixBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
    "post-api-v1-upload-sessions-by-id-resume": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SessionCommandBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SessionStatus"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HTTPError"];
                };
            };
        };
    };
}
