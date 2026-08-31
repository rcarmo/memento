#include <sqlite3ext.h>

SQLITE_EXTENSION_INIT1

int memento_sqlite_api_init(const sqlite3_api_routines *api) {
    if (api == 0) {
        return SQLITE_ERROR;
    }
    SQLITE_EXTENSION_INIT2(api);
    return SQLITE_OK;
}

int memento_sqlite_create_function_v2(
    sqlite3 *db,
    const char *name,
    int argc,
    int flags,
    void *app,
    void (*function)(sqlite3_context *, int, sqlite3_value **),
    void (*step)(sqlite3_context *, int, sqlite3_value **),
    void (*final)(sqlite3_context *),
    void (*destroy)(void *)
) {
    return sqlite3_create_function_v2(
        db, name, argc, flags, app, function, step, final, destroy
    );
}

void memento_sqlite_result_int(sqlite3_context *context, int value) {
    sqlite3_result_int(context, value);
}

void memento_sqlite_result_double(sqlite3_context *context, double value) {
    sqlite3_result_double(context, value);
}

void memento_sqlite_result_null(sqlite3_context *context) {
    sqlite3_result_null(context);
}

void memento_sqlite_result_error(
    sqlite3_context *context, const char *message, int length
) {
    sqlite3_result_error(context, message, length);
}

int memento_sqlite_value_type(sqlite3_value *value) {
    return sqlite3_value_type(value);
}

const void *memento_sqlite_value_blob(sqlite3_value *value) {
    return sqlite3_value_blob(value);
}

int memento_sqlite_value_bytes(sqlite3_value *value) {
    return sqlite3_value_bytes(value);
}
