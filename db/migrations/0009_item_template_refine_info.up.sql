-- go-metin2 migration: 0009 item_template_refine_info up
CREATE TABLE item_template_refine_infos (
    vnum BIGINT PRIMARY KEY,
    result_vnum BIGINT NOT NULL,
    cost INTEGER NOT NULL,
    probability INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (vnum) REFERENCES item_templates(vnum),
    CHECK (vnum > 0),
    CHECK (result_vnum > 0 AND result_vnum <= 4294967295),
    CHECK (cost >= 0 AND cost <= 2147483647),
    CHECK (probability >= 0 AND probability <= 100)
);

CREATE TABLE item_template_refine_materials (
    vnum BIGINT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (vnum, position),
    FOREIGN KEY (vnum) REFERENCES item_template_refine_infos(vnum),
    CHECK (vnum > 0),
    CHECK (position >= 0 AND position < 5),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (count > 0 AND count <= 2147483647)
);
