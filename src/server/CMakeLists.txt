set(KMS_ELEMENTS_IMPL_SOURCES
  implementation/objects/OV3SubscriberImpl.cpp
  implementation/objects/OV3PublisherImpl.cpp
)

set(KMS_ELEMENTS_IMPL_HEADERS
  implementation/objects/OV3SubscriberImpl.hpp
  implementation/objects/OV3PublisherImpl.hpp
)
include (CodeGenerator)

generate_code (
  MODELS ${CMAKE_CURRENT_SOURCE_DIR}/interface
  SERVER_IMPL_LIB_EXTRA_SOURCES ${KMS_ELEMENTS_IMPL_SOURCES}
  SERVER_IMPL_LIB_EXTRA_HEADERS ${KMS_ELEMENTS_IMPL_HEADERS}
  SERVER_IMPL_LIB_EXTRA_LIBRARIES ${LIBBFCP_LIBRARIES}
  SERVER_STUB_DESTINATION ${CMAKE_CURRENT_SOURCE_DIR}/implementation/objects
)

add_custom_target( maven_install  
                    COMMAND mvn install:install-file -Dfile=${CMAKE_BINARY_DIR}/java/target/ov3endpoint-1.0.0.jar -DgroupId=org.kurento.module -DartifactId=ov3endpoint -Dversion=1.0.0 -Dpackaging=jar 
                    COMMENT "Installing client library in local maven repository"
                    DEPENDS java )
