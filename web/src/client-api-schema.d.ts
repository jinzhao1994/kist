// Generated from the Go HTTP contract. Run npm run generate:api.
export interface paths {
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
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
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
        ClientCatalogStatus: {
            complete: boolean;
            confirmed: components["schemas"]["SyncBoundary"];
            /** Format: int64 */
            downloaded: number;
            transfer?: components["schemas"]["SyncBoundary"];
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
        ClientRequestResult: {
            local_id?: string;
            noop: boolean;
        };
        CreateTagInputBody: {
            name: string;
            parent_id?: string;
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
            kind?: string;
            media_type?: string;
            name: string;
            no_primary?: boolean;
            path?: string;
            primary_id?: string;
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
            holder?: string;
            message: string;
            /** Format: int64 */
            op_id?: number;
            /** Format: int64 */
            reverted_by?: number;
            /** Format: int64 */
            upload_offset?: number;
        };
        LibraryInfo: {
            /** Format: int64 */
            max_upload_bytes: number;
            /** Format: int64 */
            max_upload_files: number;
            projection_dirty: boolean;
            /** Format: int64 */
            resumable_upload_version?: number;
            timezone: string;
            /** Format: int64 */
            upload_chunk_bytes?: number;
            version: string;
        };
        MapProvider: {
            resource_origins: string[] | null;
            style_url: string;
            terrain_attribution: string;
            terrain_tiles: string;
        };
        Metadata: {
            key: string;
            source: string;
            type: string;
            value: string;
        };
        MoveTagInputBody: {
            parent_id: string | null;
        };
        "Post-api-v1-boxes-by-id-foldersRequest": {
            name: string;
            parent?: string;
        };
        "Put-api-v1-boxes-by-id-coverRequest": {
            file_id: string;
        };
        Ref: {
            id: string;
            name: string;
        };
        RenameTagInputBody: {
            name: string;
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
        SyncBoundary: {
            library_id: string;
            /** Format: int64 */
            op_id: number;
            sum: string;
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
        UpdateSavedSearchBody: {
            before: components["schemas"]["SavedSearchBody"];
            definition: components["schemas"]["SearchDefinition"];
            name: string;
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
}
