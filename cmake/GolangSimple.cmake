function(GO_MOD_INIT)
  add_custom_command(OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/go.mod
                      COMMAND rm -f ${CMAKE_CURRENT_BINARY_DIR}/go.mod
                      COMMAND rm -f ${CMAKE_CURRENT_BINARY_DIR}/*.go
                      COMMAND cp ${CMAKE_CURRENT_LIST_DIR}/*.go .
                      COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go clean -cache
                      COMMAND env GOPATH=${CMAKE_CURRENT_BINARY_DIR} go mod init ${GO_MODULE_PATH}
                      WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR})    
#  add_custom_target(GO_MOD_INIT
#                    DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/go.mod)
endfunction(GO_MOD_INIT)

function(GO_MOD_TIDY)
#  add_custom_target(GO_MOD_TIDY)
  add_custom_command(OUTPUT ./go.sum
                     COMMAND go mod tidy
                     WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR})
endfunction(GO_MOD_TIDY)

function(GO_MOD_REQUIRE NAME GO_MODULE)
  add_custom_target(${NAME} DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/go.mod)
  add_custom_command(TARGET ${NAME}
                    COMMAND go mod edit --require  ${GO_MODULE}
                    COMMAND go get -v -t ./... 
                    WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR})
endfunction(GO_MOD_REQUIRE)

function(GO_GET TARG)
  GO_MOD_TIDY()
  add_custom_target(${TARG} go get ${ARGN})
  add_dependencies(${TARG} GO_MOD_TIDY)
endfunction(GO_GET)

function(ADD_GO_INSTALLABLE_PROGRAM NAME MAIN_SRC)
  get_filename_component(MAIN_SRC_ABS ${MAIN_SRC} ABSOLUTE)
  add_custom_target(${NAME})
  add_custom_command(TARGET ${NAME}
                    COMMAND env GOPATH=${GOPATH} go build 
                    -o "${CMAKE_CURRENT_BINARY_DIR}/${NAME}"
                    ${CMAKE_GO_FLAGS} ${MAIN_SRC}
                    WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
                    DEPENDS ${MAIN_SRC_ABS} ./go.mod ./go.sum)
  foreach(DEP ${ARGN})
    add_dependencies(${NAME} ${DEP})
  endforeach()
  
  add_custom_target(${NAME}_all ALL DEPENDS ${NAME})
  install(PROGRAMS ${CMAKE_CURRENT_BINARY_DIR}/${NAME} DESTINATION bin)
endfunction(ADD_GO_INSTALLABLE_PROGRAM)

function(ADD_GO_SHARED_LIBRARY NAME MAIN_SRC DEP1)
get_filename_component(MAIN_SRC_ABS ${MAIN_SRC} ABSOLUTE)
get_filename_component(MAIN_SRC_DIR ${MAIN_SRC_ABS} DIRECTORY)
add_custom_command(OUTPUT  ${OUTPUTPATH}/lib${NAME}.so
                COMMAND rm -f ./*.go go.mod go.sum
                COMMAND cp ${MAIN_SRC_DIR}/*.go .
                COMMAND cp ${MAIN_SRC_DIR}/go.mod .
                COMMAND cp ${MAIN_SRC_DIR}/go.sum .
                COMMAND go mod tidy -v -x
                COMMAND env GOPATH=${GOPATH} 
                    CGO_CFLAGS='-O2 -g -pthread -I/usr/include/gstreamer-1.0 -I/usr/include/glib-2.0 -I/usr/lib/x86_64-linux-gnu/glib-2.0/include -lgstvideo-1.0 -lgstreamer-1.0 -lgobject-2.0 -lglib-2.0' 
                    CGO_LDFLAG='-O2 -g -lgstreamer-1.0 -lgobject-2.0 -lglib-2.0' go build 
                    -o "${OUTPUTPATH}/lib${NAME}.so"
                    -buildmode=c-shared
                    ${CMAKE_GO_FLAGS} *.go
                    WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
                    DEPENDS ${MAIN_SRC_ABS})
  add_custom_target(${NAME} ALL DEPENDS ${MAIN_SRC} ${OUTPUTPATH}/lib${NAME}.so)
#  set_target_properties(${NAME} PROPERTIES
#      TYPE SHARED_LIBRARY
#  )
  foreach(DEP ${ARGN})
    add_dependencies(${NAME} ${DEP})
  endforeach()
  
  install(PROGRAMS ${OUTPUTPATH}/lib${NAME}.so DESTINATION ${CMAKE_INSTALL_LIBDIR})
endfunction(ADD_GO_SHARED_LIBRARY)
