-- go-metin2 migration: 0022 item_template_refine_fail_result_vnum up
ALTER TABLE item_template_refine_infos
    ADD COLUMN fail_result_vnum BIGINT NOT NULL DEFAULT 0
    CHECK (
        fail_result_vnum = 0
        OR (
            fail_result_vnum > 0
            AND fail_result_vnum <= 4294967295
            AND keep_on_fail = 0
            AND probability >= 1
            AND probability <= 99
            AND fail_result_vnum != vnum
            AND fail_result_vnum != result_vnum
        )
    );
