Feature: mcp/schema tools and coercion

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_annotations.py

    @umcp-08f4eff712d5 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_read_prefix_marks_read_only
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When read prefix marks read only
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-99f835143941 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_destructive_prefix_marks_destructive
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When destructive prefix marks destructive
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-968dde3b88d9 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_open_world_prefix_marks_open_world
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When open world prefix marks open world
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c4b5bf0564fb @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_no_recognised_prefix_yields_neutral_annotations
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When no recognised prefix yields neutral annotations
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-75ee031900a3 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_explicit_mcp_annotations_override_inference
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When explicit mcp annotations override inference
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_coercion.py

    @umcp-7e650ebb8906 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_string_to_int_is_coerced
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When string to int is coerced
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-def2781caea5 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_string_to_float_is_coerced
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When string to float is coerced
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0dda614a55cc @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_string_true_to_bool_is_coerced
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When string true to bool is coerced
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-43ccc67a3c96 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_native_types_pass_through_unchanged
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When native types pass through unchanged
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_introspection.py

    @umcp-1de69fe9cf78 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_introspected_movie_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When introspected movie server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a2fff1eb6c8a @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_calculator_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When calculator server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0daefb9d4efb @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_basic_movie_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When basic movie server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_schema_fallbacks.py

    @umcp-f16004011a79 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_sync_schema_falls_back_to_signature_annotation_and_string_default
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync schema falls back to signature annotation and string default
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-fed3c6620137 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_async_schema_falls_back_to_signature_annotation_and_string_default
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async schema falls back to signature annotation and string default
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_schema_generation.py

    @umcp-2736f909764a @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_primitive_types_map_to_jsonschema
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When primitive types map to jsonschema
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d127f00e7982 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_required_vs_optional_by_default
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When required vs optional by default
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-55b34e18d68a @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_additional_properties_is_false
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When additional properties is false
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-175bd839a7e5 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_no_args_tool_gets_empty_object_schema
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When no args tool gets empty object schema
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9e04f0c1ce1a @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_optional_via_pep604_union_with_none
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When optional via pep604 union with none
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-241bd43df7e5 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_union_of_two_concrete_types_maps_to_array_of_types
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When union of two concrete types maps to array of types
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-fbab508c1a62 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_literal_becomes_enum
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When literal becomes enum
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d03e46383738 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_typeddict_required_keys_map_to_object_schema
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When typeddict required keys map to object schema
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6d6d81cb1090 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_typeddict_total_false_has_no_required_keys
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When typeddict total false has no required keys
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6e838d6eefb8 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_list_of_strings
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When list of strings
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-64ecadf74c38 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_dict_of_str_to_int
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When dict of str to int
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-909c23d4e9e9 @go_TestArgumentErrorText @go_TestCoercionAndCallErrors @go_TestHandlerPanicIsRedacted
    Scenario: test_async_base_generates_same_shape
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async base generates same shape
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_tools.py

    @umcp-c24948c22632 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_tools_list_returns_full_metadata
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When tools list returns full metadata
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-43deb75697f9 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_tools_list_includes_every_tool_method
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When tools list includes every tool method
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-94bdccadcf75 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_tool_args_section_populates_param_descriptions
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When tool args section populates param descriptions
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0cd5ea99ad49 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_string_return_wraps_as_text_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with string return wraps as text content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3fa23df60de2 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_dict_return_serialises_to_json_text
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with dict return serialises to json text
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7fc61548e318 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_list_return_serialises_to_json_text
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with list return serialises to json text
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-989b9a7059b0 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_scalar_returns_are_stringified
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with scalar returns are stringified
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-031c65d64bbe @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_none_return_serialises_as_json_null
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with none return serialises as json null
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-92bea91449db @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_uses_default_when_argument_omitted
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call uses default when argument omitted
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1119dfd14779 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_call_with_no_args_section_at_all
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When call with no args section at all
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-de44b3857c8d @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_listed_tools_can_all_be_called_round_trip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When listed tools can all be called round trip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-19089cb16f2b @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_tool_dispatch_returns_text_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tool dispatch returns text content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8ea8fab9ff49 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_tool_with_int_return
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tool with int return
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0a2444b2beef @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_base_supports_sync_tool_methods
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async base supports sync tool methods
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6820afec87ce @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_async_tools_can_run_concurrently
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tools can run concurrently
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-4504e1b19c32 @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText @go_TestCoercionAndCallErrors
    Scenario: test_async_initialize_includes_tools_capability
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async initialize includes tools capability
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
