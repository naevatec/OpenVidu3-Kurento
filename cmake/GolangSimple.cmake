function(GO_MOD_INIT)
  add_custom_command(OUTPUT ${CMAKE_CURRENT_SOURCE_DIR}/go.mod
                      COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go clean -cache
                      COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go mod init ${GO_MODULE_PATH}
                      WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})    
  add_custom_target(GO_MOD_INIT
                    DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/go.mod)
endfunction(GO_MOD_INIT)

function(GO_MOD_TIDY)
#  add_custom_target(GO_MOD_TIDY)
  add_custom_command(OUTPUT ${CMAKE_CURRENT_SOURCE_DIR}/go.sum
                     COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go mod tidy -v -x
                     WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
endfunction(GO_MOD_TIDY)

function(GO_MOD_REQUIRE NAME GO_MODULE)
  add_custom_target(${NAME} DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/go.mod)
  add_custom_command(TARGET ${NAME}
                    COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go mod edit --require  ${GO_MODULE}
                    COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go get -v -t ./... 
                    WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
endfunction(GO_MOD_REQUIRE)

function(GO_GET TARG)
  GO_MOD_TIDY()
  add_custom_target(${TARG} go get ${ARGN})
  add_dependencies(${TARG} GO_MOD_TIDY)
endfunction(GO_GET)

function(ADD_GO_INSTALLABLE_PROGRAM NAME SRCS)
  get_filename_component(SRCS_ABS ${SRCS} ABSOLUTE)
  add_custom_target(${NAME})
  add_custom_command(TARGET ${NAME}
                    COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go build 
                    -o "${CMAKE_CURRENT_BINARY_DIR}/${NAME}"
                    ${CMAKE_GO_FLAGS} ${SRCS}
                    WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
                    DEPENDS ${SRCS})
  foreach(DEPS ${ARGN})
    add_dependencies(${NAME} ${DEPS})
  endforeach()
  
  add_custom_target(${NAME}_all ALL DEPENDS ${NAME})
  install(PROGRAMS ${CMAKE_CURRENT_BINARY_DIR}/${NAME} DESTINATION bin)
endfunction(ADD_GO_INSTALLABLE_PROGRAM)

function(ADD_GO_SHARED_LIBRARY NAME OUTPUT_PATH SRCS DEPS)
  add_custom_target(${NAME} ALL 
                WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
                COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go mod tidy -v -x
                COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} 
                    CGO_CFLAGS='-O2 -g -pthread -I/usr/include/gstreamer-1.0 -I/usr/include/glib-2.0 -I/usr/lib/x86_64-linux-gnu/glib-2.0/include' 
                    CGO_LDFLAGS='-O2 -g -lgstvideo-1.0 -lgstreamer-1.0 -lgobject-2.0 -lglib-2.0' go build 
                    -o "${CMAKE_CURRENT_BINARY_DIR}/lib${NAME}.so"
                    -buildmode=c-shared
                    ${CMAKE_GO_FLAGS} ${SRCS}
                COMMAND cp "${CMAKE_CURRENT_BINARY_DIR}/lib${NAME}.so" "${OUTPUT_PATH}"
                COMMAND cp "${CMAKE_CURRENT_BINARY_DIR}/lib${NAME}.h" "${OUTPUT_PATH}"
                BYPRODUCTS ${CMAKE_CURRENT_BINARY_DIR}/lib${NAME}.so
                SOURCES "${SRCS}" 
                DEPENDS "go.mod")

  install(PROGRAMS ${OUTPUT_PATH}/lib${NAME}.so DESTINATION ${CMAKE_INSTALL_LIBDIR})
endfunction(ADD_GO_SHARED_LIBRARY)
