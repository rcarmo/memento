fn main() {
    cc::Build::new()
        .file("src/sqlite_api.c")
        .warnings(true)
        .compile("memento_sqlite_api");
    println!("cargo:rerun-if-changed=src/sqlite_api.c");
}
