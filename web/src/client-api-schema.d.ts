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
    "/api/v1/client/cache": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client cache */
        get: operations["get-api-v1-client-cache"];
        /** Put API v1 client cache */
        put: operations["put-api-v1-client-cache"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/cache/prune": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client cache prune */
        post: operations["post-api-v1-client-cache-prune"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client catalog */
        get: operations["get-api-v1-client-catalog"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/connection": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 client connection */
        put: operations["put-api-v1-client-connection"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/connection-status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client connection status */
        get: operations["get-api-v1-client-connection-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/history": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client history */
        get: operations["get-api-v1-client-history"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/history/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client history by ID */
        get: operations["get-api-v1-client-history-by-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/history/{id}/undo": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client history by ID undo */
        post: operations["post-api-v1-client-history-by-id-undo"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/offline": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client offline */
        get: operations["get-api-v1-client-offline"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/offline/{owner}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client offline by owner */
        get: operations["get-api-v1-client-offline-by-owner"];
        /** Put API v1 client offline by owner */
        put: operations["put-api-v1-client-offline-by-owner"];
        post?: never;
        /** Delete API v1 client offline by owner */
        delete: operations["delete-api-v1-client-offline-by-owner"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/offline/{owner}/prepare": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client offline by owner prepare */
        post: operations["post-api-v1-client-offline-by-owner-prepare"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/pending-history": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client pending history */
        get: operations["get-api-v1-client-pending-history"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/pending/{id}/edit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client pending by ID edit */
        get: operations["get-api-v1-client-pending-by-id-edit"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/pending/{id}/resolve": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client pending by ID resolve */
        post: operations["post-api-v1-client-pending-by-id-resolve"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/protected-storage": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client protected storage */
        get: operations["get-api-v1-client-protected-storage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 client status */
        get: operations["get-api-v1-client-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/sync": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client sync */
        post: operations["post-api-v1-client-sync"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/client/sync/automatic": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 client sync automatic */
        post: operations["post-api-v1-client-sync-automatic"];
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
    "/api/v1/files/{id}/metadata": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Put API v1 files by ID metadata */
        put: operations["put-api-v1-files-by-id-metadata"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
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
    "/api/v1/sync/relay/catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["relay-catalog"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/relay/head": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["relay-head"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync/relay/submit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["relay-submit"];
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
            /** @enum {string} */
            readonly authentication_error?: "" | "temporary" | "reauthenticate";
            clear_secret?: boolean;
            readonly connected: boolean;
            readonly email: string;
            id: string;
            login_draft?: string;
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
        ClientCacheSettingsBody: {
            max_age: string;
            /** Format: int64 */
            max_bytes: number;
        };
        ClientCacheStatus: {
            /** Format: int64 */
            cached_bytes: number;
            /** Format: int64 */
            corrupt_bytes: number;
            /** Format: int64 */
            limit_bytes: number;
            over_limit: boolean;
            /** Format: int64 */
            partial_bytes: number;
            /** Format: int64 */
            pinned_bytes: number;
            /** Format: int64 */
            reserved_bytes: number;
        };
        ClientCatalogStatus: {
            complete: boolean;
            confirmed: components["schemas"]["SyncBoundary"];
            /** Format: int64 */
            downloaded: number;
            transfer?: components["schemas"]["SyncBoundary"];
        };
        ClientConflictCover: {
            before: components["schemas"]["ClientConflictCoverFile"];
            current: components["schemas"]["ClientConflictCoverFile"];
            proposed: components["schemas"]["ClientConflictCoverFile"];
        };
        ClientConflictCoverFile: {
            available: boolean;
            id: string;
            name: string;
        };
        ClientConflictDeletion: {
            before: components["schemas"]["ClientDeletionPreview"];
            box_id: string;
            current?: components["schemas"]["ClientDeletionPreview"];
            folder: boolean;
            id: string;
            whole_box: boolean;
        };
        ClientConflictEdit: {
            cover?: components["schemas"]["ClientConflictCover"];
            deletion?: components["schemas"]["ClientConflictDeletion"];
            fields: components["schemas"]["ClientConflictField"][] | null;
            fileset?: components["schemas"]["ClientConflictFileset"];
            replacement?: components["schemas"]["SyncProposal"];
            summary?: components["schemas"]["HistorySummary"];
        };
        ClientConflictField: {
            before: string;
            current: string;
            /** @enum {string} */
            key: "name" | "description" | "start_at" | "end_at";
            path: string[] | null;
            proposed: string;
        };
        ClientConflictFileset: {
            before?: components["schemas"]["Fileset"];
            box_id: string;
            current?: components["schemas"]["Fileset"];
            folder_id: string;
            name: string;
            proposed?: components["schemas"]["Fileset"];
        };
        ClientConnectInputBody: {
            allow_http: boolean;
            revision?: string;
            server: string;
            token: string;
        };
        ClientConnection: {
            allow_http: boolean;
            server: string;
        };
        ClientConnectionStatus: {
            /** Format: int64 */
            awaiting_root: number;
            catalog_complete: boolean;
            /** Format: int64 */
            catalog_op_id: number;
            manual_required: boolean;
            /** Format: int64 */
            pending_download?: number;
            /** Format: int64 */
            pending_upload: number;
            progress?: components["schemas"]["ClientSyncProgress"];
            /** Format: int64 */
            relay_sequence: number;
            relay_updates: boolean;
            server?: string;
            /** @enum {string} */
            state: "connected" | "unreachable" | "unauthorized" | "incompatible" | "unconfigured";
            syncing: boolean;
        };
        ClientDeletionPreview: {
            /** Format: int64 */
            file_count: number;
            /** Format: int64 */
            folder_count: number;
            name: string;
            revision: string;
        };
        ClientEntryDeleteInputBody: {
            revision: string;
        };
        ClientLocalHistoryDetail: {
            action: string;
            group?: components["schemas"]["SyncReference"];
            id: string;
            /** Format: date-time */
            occurred_at: string;
            origin: string;
            proposal: components["schemas"]["SyncProposal"];
            reason?: string;
            receipt?: components["schemas"]["SyncResult"];
            replaced_by?: string;
            /** @enum {string} */
            state: "pending" | "awaiting_receipt" | "awaiting_root" | "conflict" | "accepted" | "discarded" | "replaced";
            summary?: components["schemas"]["HistorySummary"];
            undo?: components["schemas"]["SyncReference"];
            undone_by?: components["schemas"]["SyncReference"];
        };
        ClientLocalHistoryEntry: {
            action: string;
            group?: components["schemas"]["SyncReference"];
            id: string;
            /** Format: date-time */
            occurred_at: string;
            origin: string;
            reason?: string;
            receipt?: components["schemas"]["SyncResult"];
            replaced_by?: string;
            /** @enum {string} */
            state: "pending" | "awaiting_receipt" | "awaiting_root" | "conflict" | "accepted" | "discarded" | "replaced";
            summary?: components["schemas"]["HistorySummary"];
            undo?: components["schemas"]["SyncReference"];
            undone_by?: components["schemas"]["SyncReference"];
        };
        ClientLocalHistoryList: {
            before?: string;
            entries: components["schemas"]["ClientLocalHistoryEntry"][] | null;
        };
        ClientOfflineOriginal: {
            files: string[] | null;
            key: string;
            /** Format: int64 */
            size: number;
        };
        ClientOfflinePreview: {
            available: boolean;
            files: string[] | null;
            key: string;
            recipe: string;
        };
        ClientOfflineSelection: {
            files: string[] | null;
            filesets: string[] | null;
        };
        ClientOfflineStatus: {
            /** Format: int64 */
            available: number;
            catalog: components["schemas"]["ClientCatalogStatus"];
            missing: string[] | null;
            originals: components["schemas"]["ClientOfflineOriginal"][] | null;
            previews: components["schemas"]["ClientOfflinePreview"][] | null;
            /** Format: int64 */
            previews_available: number;
            ready: boolean;
        };
        ClientPendingStatus: {
            action: string;
            awaiting_receipt: boolean;
            awaiting_root: boolean;
            conflict?: string;
            id: string;
            /** Format: date-time */
            occurred_at: string;
            /** Format: int64 */
            sequence: number;
        };
        ClientProtectedStorage: {
            /** Format: int64 */
            originals_bytes: number;
            /** Format: int64 */
            pending_journal_bytes: number;
        };
        ClientRequestResult: {
            local_id?: string;
            noop: boolean;
        };
        ClientSettings: {
            cache_max_age: string;
            /** Format: int64 */
            cache_max_bytes: number;
            id: string;
            library_id: string;
            timezone: string;
            /** Format: int64 */
            version: number;
        };
        ClientStatus: {
            blocked_id?: string;
            catalog: components["schemas"]["ClientCatalogStatus"];
            connection?: components["schemas"]["ClientConnection"];
            pending: components["schemas"]["ClientPendingStatus"][] | null;
            settings: components["schemas"]["ClientSettings"];
        };
        ClientSyncProgress: {
            /** Format: int64 */
            count: number;
            /** @enum {string} */
            direction: "download" | "upload";
        };
        ClientSyncResponse: {
            result: components["schemas"]["ClientSyncResult"];
            status: components["schemas"]["ClientStatus"];
        };
        ClientSyncResult: {
            /** Format: int64 */
            accepted: number;
            /** Format: int64 */
            awaiting_root?: number;
            blocked_id?: string;
            complete: boolean;
            manual_required?: boolean;
            /** Format: int64 */
            remaining: number;
        };
        CreateTagInputBody: {
            name: string;
            parent_id?: string;
        };
        CredentialDraft: {
            id: string;
            note: string;
        };
        CredentialSettings: {
            items: components["schemas"]["CredentialDraft"][] | null;
        };
        "Delete-api-v1-boxes-by-idRequest": {
            revision: string;
        };
        DisplayTag: {
            id: string;
            name: string;
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
        ExtractionInputBody: {
            force?: boolean;
        };
        Feature: {
            /** Format: int64 */
            account: number;
            automatic: boolean;
            image_detail: string;
            model: string;
            pause_on_error?: boolean;
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
        "OutputStruct {}Body": Record<string, never>;
        Point: {
            /** Format: double */
            accuracy?: number;
            /** Format: double */
            latitude: number;
            /** Format: double */
            longitude: number;
        };
        "Post-api-v1-boxes-by-id-foldersRequest": {
            name: string;
            parent?: string;
        };
        "Post-api-v1-client-cache-pruneRequest": Record<string, never>;
        "Post-api-v1-client-history-by-id-undoRequest": Record<string, never>;
        "Post-api-v1-client-offline-by-owner-prepareRequest": Record<string, never>;
        "Post-api-v1-client-pending-by-id-resolveRequest": {
            reason: string;
            replacement?: components["schemas"]["SyncProposal"];
        };
        "Post-api-v1-client-sync-automaticRequest": Record<string, never>;
        "Post-api-v1-client-syncRequest": Record<string, never>;
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
        "Put-api-v1-settings-credentials-by-idRequest": {
            note: string;
        };
        Ref: {
            id: string;
            name: string;
        };
        "Relay-catalogRequest": {
            /** Format: int64 */
            after: number;
            head: components["schemas"]["RelayHead"];
            /** Format: int64 */
            limit: number;
            /** Format: int64 */
            max_bytes: number;
        };
        "Relay-submitRequest": {
            expected?: components["schemas"]["RelayHead"];
            library_id: string;
            proposal: components["schemas"]["SyncProposal"];
        };
        RelayHead: {
            /** Format: int64 */
            count: number;
            node: string;
            root: components["schemas"]["SyncBoundary"];
            /** Format: int64 */
            sequence: number;
            sum: string;
        };
        RelayItem: {
            confirmed?: components["schemas"]["SyncResult"];
            conflict?: string;
            discarded: boolean;
            proposal: components["schemas"]["SyncProposal"];
            reason?: string;
            replaced_by?: string;
        };
        RelayOutcome: {
            confirmed?: components["schemas"]["SyncResult"];
            conflict?: string;
            custody: components["schemas"]["RelayReceipt"];
            discarded: boolean;
            reason?: string;
            replacement?: components["schemas"]["SyncProposal"];
            sending: boolean;
        };
        RelayPage: {
            /** Format: int64 */
            after: number;
            done: boolean;
            head: components["schemas"]["RelayHead"];
            items: components["schemas"]["RelayItem"][] | null;
            /** Format: int64 */
            next: number;
        };
        RelayReceipt: {
            digest: string;
            library_id: string;
            proposal_id: string;
            /** Format: date-time */
            received_at: string;
            relay_id: string;
        };
        RelaySubmitResponse: {
            error?: components["schemas"]["Error"];
            rejected: boolean;
            result?: components["schemas"]["RelayOutcome"];
        };
        RenameTagInputBody: {
            name: string;
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
            credentials?: components["schemas"]["CredentialSettings"];
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
        SettingsSavedResponse: {
            connection?: components["schemas"]["ClientConnection"];
            created_credentials?: components["schemas"]["NewSyncCredential"][] | null;
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                "application/json": components["schemas"]["ClientEntryDeleteInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientDeletionPreview"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientDeletionPreview"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
    "get-api-v1-client-cache": {
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
                    "application/json": components["schemas"]["ClientCacheStatus"];
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
    "put-api-v1-client-cache": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ClientCacheSettingsBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientCacheStatus"];
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
    "post-api-v1-client-cache-prune": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-client-cache-pruneRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientCacheStatus"];
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
    "get-api-v1-client-catalog": {
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
                    "application/json": components["schemas"]["ClientCatalogStatus"];
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
    "put-api-v1-client-connection": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ClientConnectInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientStatus"];
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
    "get-api-v1-client-connection-status": {
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
                    "application/json": components["schemas"]["ClientConnectionStatus"];
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
    "get-api-v1-client-history": {
        parameters: {
            query?: {
                before?: string;
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
                    "application/json": components["schemas"]["ClientLocalHistoryList"];
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
    "get-api-v1-client-history-by-id": {
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
                    "application/json": components["schemas"]["ClientLocalHistoryDetail"];
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
    "post-api-v1-client-history-by-id-undo": {
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
                "application/json": components["schemas"]["Post-api-v1-client-history-by-id-undoRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientRequestResult"];
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
    "get-api-v1-client-offline": {
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
                    "application/json": {
                        [key: string]: components["schemas"]["ClientOfflineSelection"];
                    };
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
    "get-api-v1-client-offline-by-owner": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                owner: string;
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
                    "application/json": components["schemas"]["ClientOfflineStatus"];
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
    "put-api-v1-client-offline-by-owner": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                owner: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ClientOfflineSelection"];
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
    "delete-api-v1-client-offline-by-owner": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                owner: string;
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
    "post-api-v1-client-offline-by-owner-prepare": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                owner: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-client-offline-by-owner-prepareRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientOfflineStatus"];
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
    "get-api-v1-client-pending-history": {
        parameters: {
            query?: {
                box?: string;
                file?: string;
                fileset?: string;
                trash?: boolean;
                before?: string;
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
                    "application/json": components["schemas"]["ClientLocalHistoryList"];
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
    "get-api-v1-client-pending-by-id-edit": {
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
                    "application/json": components["schemas"]["ClientConflictEdit"];
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
    "post-api-v1-client-pending-by-id-resolve": {
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
                "application/json": components["schemas"]["Post-api-v1-client-pending-by-id-resolveRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientStatus"];
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
    "get-api-v1-client-protected-storage": {
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
                    "application/json": components["schemas"]["ClientProtectedStorage"];
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
    "get-api-v1-client-status": {
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
                    "application/json": components["schemas"]["ClientStatus"];
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
    "post-api-v1-client-sync": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-client-syncRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ClientSyncResponse"];
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
    "post-api-v1-client-sync-automatic": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Post-api-v1-client-sync-automaticRequest"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["SettingsSavedResponse"];
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
    "relay-catalog": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Relay-catalogRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RelayPage"];
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
    "relay-head": {
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
                    "application/json": components["schemas"]["RelayHead"];
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
    "relay-submit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Relay-submitRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RelaySubmitResponse"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
                    "application/json": components["schemas"]["ClientRequestResult"];
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
