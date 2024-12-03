#define BOOST_TEST_STATIC_LINK
#define BOOST_TEST_PROTECTED_VIRTUAL

#include <boost/test/included/unit_test.hpp>
#include <MediaPipelineImpl.hpp>
#include <ModuleManager.hpp>
#include <KurentoException.hpp>
#include <MediaSet.hpp>
#include <MediaElementImpl.hpp>
#include <RecorderEndpointImpl.hpp>
#include <MediaProfileSpecType.hpp>
#include <modules/filters/GStreamerFilter.hpp>
#include <HubPortImpl.hpp>
#include <CompositeImpl.hpp>
#include <ConnectionState.hpp>
#include <MediaFlowInStateChanged.hpp>
#include <MediaFlowState.hpp>
#include <MediaType.hpp>
#include <GstreamerDotDetails.hpp>
#include <sigc++/connection.h>

#include <RegisterParent.hpp>

#include <PassThroughImpl.hpp>

#include "OV3SubscriberImpl.hpp"
#include "OV3PublisherImpl.hpp"

kurento::ModuleManager moduleManager;

namespace kurento
{
  ModuleManager& getModuleManager ();
}


kurento::ModuleManager& kurento::getModuleManager ()
{
  return moduleManager;
}



using namespace kurento;
using namespace boost::unit_test;

boost::property_tree::ptree config;
unsigned short int bfcpPort = 2345;

struct GF {
  GF();
  ~GF();
};

BOOST_GLOBAL_FIXTURE (GF);

GF::GF()
{
  boost::property_tree::ptree ac, audioCodecs, vc, videoCodecs;
  gst_init(nullptr, nullptr);
//  moduleManager.loadModulesFromDirectories ("./src/server:../../kms-omni-build:../../src/server:../../../../kms-omni-build");
  moduleManager.loadModulesFromDirectories ("../../src/server:./");

  config.add ("configPath", "../../../tests" );
  config.add ("modules.kurento.SdpEndpoint.numAudioMedias", 1);
  config.add ("modules.kurento.SdpEndpoint.numVideoMedias", 1);

  ac.put ("name", "opus/48000/2");
  audioCodecs.push_back (std::make_pair ("", ac) );
  config.add_child ("modules.kurento.SdpEndpoint.audioCodecs", audioCodecs);

  vc.put ("name", "H264/90000");
  videoCodecs.push_back (std::make_pair ("", vc) );
  config.add_child ("modules.kurento.SdpEndpoint.videoCodecs", videoCodecs);
  config.add ("modules.bfcp.BfcpSession.BFCPListenPort", bfcpPort);

}

GF::~GF()
{
  sleep (3);
  //MediaSet::deleteMediaSet();
}

static void
dumpPipeline (std::shared_ptr<MediaPipeline> pipeline, std::string fileName)
{
  std::string pipelineDot;
  std::shared_ptr<GstreamerDotDetails> details (new GstreamerDotDetails ("SHOW_ALL"));

  pipelineDot = pipeline->getGstreamerDot (details);
  std::ofstream out(fileName);

  out << pipelineDot;
  out.close ();

}

void
dumpPipeline (std::string pipelineId, std::string fileName)
{
  std::shared_ptr<MediaPipeline> pipeline = std::dynamic_pointer_cast<MediaPipeline> (MediaSet::getMediaSet ()->getMediaObject (pipelineId));
  dumpPipeline (pipeline, fileName);

//  MediaSet::getMediaSet ()->release (pipelineId);
}

static void
checkPipelineEmpty (std::string pipelineId)
{
  std::shared_ptr<MediaPipeline> pipeline = std::dynamic_pointer_cast<MediaPipeline> (MediaSet::getMediaSet ()->getMediaObject (pipelineId));
  std::vector<std::shared_ptr<MediaObject> > elements =  pipeline->getChildren ();
  std::stringstream strStream;

  for (auto & elem : elements) {
    BOOST_TEST_MESSAGE ("Still Media Element alive");
    BOOST_TEST_MESSAGE (elem->getId ());
  }

  if (!elements.empty ()) {
    BOOST_TEST_MESSAGE ("It should not remain any media element");
  } else {
    BOOST_TEST_MESSAGE ("Media pipeline empty");
  }

  strStream << pipelineId << "_final.dot";
  dumpPipeline (pipeline, strStream.str ());

  //MediaSet::getMediaSet ()->release (pipeline->getId ());
}

static std::string
createPipeline ()
{
  return moduleManager.getFactory ("MediaPipeline")->createObject (
                      config, "",
                      Json::Value() )->getId();
}

static void
releasePipeline (std::string pipelineId)
{
  MediaSet::getMediaSet ()->release (pipelineId);

}

static std::shared_ptr<HubPortImpl>
createPort (std::shared_ptr<CompositeImpl> composite)
{
  std::string id = std::dynamic_pointer_cast<MediaObject> (composite)->getId();
  std::shared_ptr <kurento::MediaObjectImpl> port;
  Json::Value constructorParams;

  constructorParams ["hub"] = id.c_str();



  port = moduleManager.getFactory ("HubPort")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <HubPortImpl> (port);

}


static void
releasePort (std::shared_ptr<HubPortImpl> port)
{
  std::string id = std::dynamic_pointer_cast<MediaElement> (port)->getId();

  port.reset();
  MediaSet::getMediaSet ()->release (id);
}


static std::shared_ptr<CompositeImpl>
createComposite (std::string mediaPipelineId)
{
  std::shared_ptr <kurento::MediaObjectImpl> composite;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;


  composite = moduleManager.getFactory ("Composite")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <CompositeImpl> (composite);

}

static void
releaseComposite (std::shared_ptr<CompositeImpl> composite)
{
  std::string id = std::dynamic_pointer_cast<MediaElement> (composite)->getId();

  composite.reset();
  MediaSet::getMediaSet ()->release (id);
}

static std::shared_ptr<OV3SubscriberImpl>
createOv3Subscriber (std::string mediaPipelineId)
{
  std::shared_ptr <kurento::MediaObjectImpl> ov3Subscriber;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;
  constructorParams ["url"] = "https://livekit.mymeeting-dev.naevatec.com:443";
  constructorParams ["key"] = "APIjJf7zm7zxqgJ";
  constructorParams ["secret"] = "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj";


  ov3Subscriber = moduleManager.getFactory ("OV3Subscriber")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <OV3SubscriberImpl> (ov3Subscriber);
}

static void
releaseOv3Subscriber (std::shared_ptr<OV3SubscriberImpl> subscriber)
{
  std::string id = std::dynamic_pointer_cast<MediaElement> (subscriber)->getId();

  subscriber.reset();
  MediaSet::getMediaSet ()->release (id);
}

static std::shared_ptr<OV3PublisherImpl>
createOv3Publisher (std::string mediaPipelineId, bool screenShare)
{
  std::shared_ptr <kurento::MediaObjectImpl> ov3Publisher;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;
  constructorParams ["url"] = "https://livekit.mymeeting-dev.naevatec.com:443";
  constructorParams ["key"] = "APIjJf7zm7zxqgJ";
  constructorParams ["secret"] = "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj";
  constructorParams ["room"] = "7tps-vk8m";
  constructorParams ["participantId"] = "test";
  constructorParams ["participantName"] = "test";
  constructorParams ["screenShare"] = screenShare ;


  ov3Publisher = moduleManager.getFactory ("Ov3Publisher")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <OV3PublisherImpl> (ov3Publisher);
}

static void
releaseOv3Publisher (std::shared_ptr<OV3PublisherImpl> publisher)
{
  std::string id = std::dynamic_pointer_cast<MediaElement> (publisher)->getId();

  publisher.reset();
  MediaSet::getMediaSet ()->release (id);
}


static std::shared_ptr<RecorderEndpointImpl>
createRecorder (std::string mediaPipelineId, std::string filename)
{
  std::shared_ptr <kurento::MediaObjectImpl> ov3subscriber;
  Json::Value constructorParams;
  std::string type;

  if (filename.find_last_of(".webm") > 0) {
    type = "WEBM";
  } else if (filename.find_last_not_of(".mp4") > 0) {
    type = "MP4";
  } else {
    return nullptr;
  }

  constructorParams ["mediaPipeline"] = mediaPipelineId;
  constructorParams ["uri"] = filename.c_str();
  constructorParams ["mediaProfile"] = type;
  constructorParams ["stopOnEndOfStream"] = true;


  ov3subscriber = moduleManager.getFactory ("RecorderEndpoint")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <RecorderEndpointImpl> (ov3subscriber);
}

static void
releaseRecorder (std::shared_ptr<RecorderEndpointImpl> recorder)
{
  std::string id = std::dynamic_pointer_cast<MediaElement> (recorder)->getId();

  recorder.reset();
  MediaSet::getMediaSet ()->release (id);
}


static std::shared_ptr <PassThroughImpl>
createPassThrough (std::string mediaPipelineId)
{
  std::shared_ptr <kurento::MediaObjectImpl> pt;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;

  pt = moduleManager.getFactory ("PassThrough")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <PassThroughImpl> (pt);
}

static void
releasePassTrhough (std::shared_ptr<PassThroughImpl> &ep)
{
  std::string id = ep->getId();

  ep.reset();
  MediaSet::getMediaSet ()->release (id);
}

std::shared_ptr<MediaElementImpl> 
createGstreamerFilter (std::string mediaPipelineId, std::string filterLine)
{
  std::shared_ptr <kurento::MediaObjectImpl> filter;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;
  constructorParams ["command"] = filterLine;

  filter = moduleManager.getFactory ("GStreamerFilter")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <MediaElementImpl> (filter);
}

void
releaseGstreamerFilter (std::shared_ptr<MediaElementImpl>  filter)
{
  if (filter == nullptr) {
    return;
  }
  std::string id = filter->getId();

  filter.reset();
  MediaSet::getMediaSet ()->release (id);
}


/*static std::shared_ptr<MediaElement>
createWhepPublisher (std::string mediaPipelineId, std::string id)
{
  std::shared_ptr <kurento::MediaObjectImpl> whepPublisher;
  Json::Value constructorParams;

  constructorParams ["mediaPipeline"] = mediaPipelineId;
  constructorParams ["webId"] = id.c_str();

  whepPublisher = moduleManager.getFactory ("WhepPublisherEndpoint")->createObject (
                  config, "",
                  constructorParams );

  return std::dynamic_pointer_cast <MediaElement> (whepPublisher);
}*/

static void
test_subscriber (bool screenShare)
{
  std::atomic<bool> media_state_changed (false);
  std::condition_variable cv;
  std::mutex mtx;
  std::unique_lock<std::mutex> lck (mtx);

  std::string pipelineId = createPipeline ();

  std::shared_ptr<OV3SubscriberImpl> subscriber = createOv3Subscriber (pipelineId);
  std::shared_ptr<PassThroughImpl> passthrough = createPassThrough (pipelineId);

  if (subscriber == nullptr) {
      BOOST_ERROR ("Could not create subscribner");
  } else if (subscriber->subscribeParticipant("7tps-vk8m", "saul", screenShare)) {
      BOOST_TEST_MESSAGE ("Ov3Subscriber created");
      subscriber->connect (passthrough);

      if (!subscriber->getIsConnected ()) {
        BOOST_ERROR("Subscribers could not connect");
      }
      sigc::connection conn = passthrough->signalMediaFlowInStateChanged.connect ([&] (MediaFlowInStateChanged event) {
        BOOST_TEST_MESSAGE ("Ov3Subscriber flowing");
        if ((event.getMediaType()->getValue() == MediaType::VIDEO ) && ((event.getState()->getValue()) == MediaFlowState::FLOWING)) {
          media_state_changed = true;
          cv.notify_one ();
        }
      });
      sleep(3);
      dumpPipeline (pipelineId, "ov3subscriber2.dot");
      cv.wait (lck, [&] () {
        return media_state_changed.load ();
      });
      dumpPipeline (pipelineId, "ov3subscriber.dot");

      subscriber->disconnect (passthrough);
      releasePassTrhough (passthrough);
      releaseOv3Subscriber (subscriber);
  } else  {
      BOOST_ERROR ("Could not make subscription");
      releasePassTrhough (passthrough);
      releaseOv3Subscriber (subscriber);
  }
  checkPipelineEmpty (pipelineId);
  releasePipeline (pipelineId);
}


static void
test_republish (bool screenShare)
{
  std::atomic<bool> media_state_changed (false);
  std::condition_variable cv;
  std::mutex mtx;
  std::unique_lock<std::mutex> lck (mtx);

  std::string pipelineId = createPipeline ();

  std::shared_ptr<OV3SubscriberImpl> subscriber = createOv3Subscriber (pipelineId);
  //std::shared_ptr<PassThroughImpl> passthrough = createPassThrough (pipelineId);

  std::shared_ptr<OV3PublisherImpl> publisher = createOv3Publisher (pipelineId, screenShare);

  std::shared_ptr<MediaElementImpl> audioDecoder = createGstreamerFilter(pipelineId, "capsfilter caps=audio/x-raw");

  subscriber->connect (audioDecoder);
  audioDecoder->connect (publisher);
  sigc::connection conn = audioDecoder->signalMediaFlowInStateChanged.connect ([&] (MediaFlowInStateChanged event) {
    BOOST_TEST_MESSAGE ("OV3Subscriber flowing");
    if ((event.getMediaType()->getValue() == MediaType::VIDEO ) && ((event.getState()->getValue()) == MediaFlowState::FLOWING)) {
      media_state_changed = true;
      cv.notify_one ();
    }
  });

  if ((subscriber == nullptr) || (publisher == nullptr)) {
      BOOST_ERROR ("Could not create subscribner");
  } else if (subscriber->subscribeParticipant("7tps-vk8m", "saul", screenShare)) {
      BOOST_TEST_MESSAGE ("OV3Subscriber created");

      if (!subscriber->getIsConnected ()) {
        BOOST_ERROR("Subscribers could not connect");
      }

      publisher->publishParticipant();
      sleep(10);
      dumpPipeline (pipelineId, "ov3subscriber2.dot");
      cv.wait (lck, [&] () {
        return media_state_changed.load ();
      });
      sleep(2);
      dumpPipeline (pipelineId, "ov3subscriber.dot");

      subscriber->disconnect (audioDecoder);
      audioDecoder->disconnect(publisher);
      //releasePassTrhough (passthrough);
      releaseGstreamerFilter(audioDecoder);
      releaseOv3Subscriber (subscriber);
      releaseOv3Publisher (publisher);
  } else  {
      BOOST_ERROR ("Could not make subscription");
      //releasePassTrhough (passthrough);
      releaseGstreamerFilter(audioDecoder);
      releaseOv3Subscriber (subscriber);
      releaseOv3Publisher (publisher);
  }
  sleep(20);
  dumpPipeline (pipelineId, "ov3subscriber_empty.dot");
  checkPipelineEmpty (pipelineId);
  releasePipeline (pipelineId);
}


static void
test_subscriber2 ()
{
  std::atomic<bool> media_state_changed (false);
  std::condition_variable cv;
  std::mutex mtx;
  std::unique_lock<std::mutex> lck (mtx);

  std::atomic<bool> media_state_changedSS (false);
  std::condition_variable cvSS;
  std::mutex mtxSS;
  std::unique_lock<std::mutex> lckSS (mtxSS);

  std::string pipelineId = createPipeline ();

  std::shared_ptr<OV3SubscriberImpl> subscriber = createOv3Subscriber (pipelineId);
  std::shared_ptr<PassThroughImpl> passthrough = createPassThrough (pipelineId);
  std::shared_ptr<RecorderEndpointImpl> recorder = createRecorder(pipelineId, "file:///tmp/test_recordinput.webm");
  std::shared_ptr<RecorderEndpointImpl> recorderMp4 = createRecorder(pipelineId, "file:///tmp/test_recordoutput.webm");
  std::shared_ptr<RecorderEndpointImpl> recorderMp4_2 = createRecorder(pipelineId, "file:///tmp/test_record2.mp4");
  std::shared_ptr<MediaElementImpl> capsfilter = createGstreamerFilter (pipelineId, "capsfilter name=audio-coding-filter caps=audio/x-raw");
  std::shared_ptr<CompositeImpl> compositor = createComposite(pipelineId);
  std::shared_ptr<HubPortImpl> sourcePort = createPort(compositor);
  std::shared_ptr<HubPortImpl> sourcePort2 = createPort(compositor);
  std::shared_ptr<HubPortImpl> sinkPort = createPort(compositor);

  std::shared_ptr<OV3SubscriberImpl> subscriberSS = createOv3Subscriber (pipelineId);
  std::shared_ptr<PassThroughImpl> passthroughSS = createPassThrough (pipelineId);



  subscriber->connect (passthrough);
  passthrough->connect(recorder);
  passthrough->connect(capsfilter);
  capsfilter->connect(sourcePort);
  capsfilter->connect(recorderMp4_2);
  //passthrough->connect(sourcePort);
  sinkPort->connect(recorderMp4);
  sigc::connection conn = passthrough->signalMediaFlowInStateChanged.connect ([&] (MediaFlowInStateChanged event) {
    if (event.getMediaType()->getValue() == MediaType::VIDEO ) {
      if (event.getState()->getValue() == MediaFlowState::FLOWING) {
        BOOST_TEST_MESSAGE ("OV3Subscriber video flowing");
        media_state_changed = true;
        cv.notify_one ();
      }else {
          BOOST_TEST_MESSAGE ("OV3Subscriber video NOT flowing");
      }
    } else if (event.getMediaType()->getValue() == MediaType::AUDIO ) {
      if (event.getState()->getValue() == MediaFlowState::FLOWING) {
        BOOST_TEST_MESSAGE ("OV3Subscriber audio flowing");
        media_state_changed = true;
        cv.notify_one ();
      }else {
          BOOST_TEST_MESSAGE ("OV3Subscriber audio NOT flowing");
      }
    }
  });
  passthroughSS->connect(sourcePort2, std::shared_ptr<MediaType>(new MediaType(MediaType::AUDIO)));
  sigc::connection conn2 = passthroughSS->signalMediaFlowInStateChanged.connect ([&] (MediaFlowInStateChanged event) {
    if (event.getMediaType()->getValue() == MediaType::VIDEO) {
      if ((event.getState()->getValue()) == MediaFlowState::FLOWING) {
        BOOST_TEST_MESSAGE ("2nd OV3Subscriber video flowing");
        media_state_changedSS = true;
        cvSS.notify_one ();
      } else {
        BOOST_TEST_MESSAGE ("2nd OV3Subscriber video NOT flowing");
      }
    }else if (event.getMediaType()->getValue() == MediaType::AUDIO) {
      if ((event.getState()->getValue()) == MediaFlowState::FLOWING) {
        BOOST_TEST_MESSAGE ("2nd OV3Subscriber audio flowing");
        media_state_changedSS = true;
        cvSS.notify_one ();
      } else {
        BOOST_TEST_MESSAGE ("2nd OV3Subscriber audio NOT flowing");
      }
    }
  });
  if ((subscriber == nullptr) || (subscriberSS == nullptr)) {
      BOOST_ERROR ("Could not create subscribner");
  } else if (subscriber->subscribeParticipant("7tps-vk8m", "saul", false)) {
      BOOST_TEST_MESSAGE ("OV3Subscriber created");

      if (!subscriber->getIsConnected ()) {
        BOOST_ERROR("Subscribers could not connect");
      }
      recorder->record();
      recorderMp4->record();
      recorderMp4_2->record();
      cv.wait (lck, [&] () {
        return media_state_changed.load ();
      });
      sleep(30);
      dumpPipeline (pipelineId, "ov3subscriber_init.dot");
      recorder->stopAndWait();
      recorderMp4->stopAndWait();
      recorderMp4_2->stopAndWait();

      if (subscriberSS->subscribeParticipant("7tps-vk8m", "saul", true)) {
        dumpPipeline (pipelineId, "ov3subscribera.dot");
        BOOST_TEST_MESSAGE ("2nd OV3Subscriber created");
        subscriberSS->connect (passthroughSS);

        if (!subscriberSS->getIsConnected ()) {
          BOOST_ERROR("Subscribers could not connect");
        }
        dumpPipeline (pipelineId, "ov3subscriberb.dot");
        cvSS.wait (lck, [&] () {
          return media_state_changedSS.load ();
        });
        dumpPipeline (pipelineId, "ov3subscriberc.dot");
        sleep(30);

        subscriberSS->disconnect (passthroughSS);
        passthroughSS->disconnect(sourcePort2);
        releasePassTrhough (passthroughSS);
        releaseOv3Subscriber (subscriberSS);
      }

      sinkPort->disconnect(recorderMp4);
      passthrough->disconnect(recorder);
      //passthrough->disconnect(recorderMp4);
      passthrough->disconnect(sourcePort);
      subscriber->disconnect (passthrough);
      releaseRecorder(recorder);
      releaseRecorder(recorderMp4);
      releasePassTrhough (passthrough);
      releaseOv3Subscriber (subscriber);
      releasePort(sinkPort);
      releasePort(sourcePort);
      releasePort(sourcePort2);
      releaseComposite(compositor);
  } else  {
      BOOST_ERROR ("Could not make subscription");
      releasePassTrhough (passthrough);
      releaseOv3Subscriber (subscriber);
  }
  checkPipelineEmpty (pipelineId);
  releasePipeline (pipelineId);
}


static void
test_mixer_sub_pub (bool screenShare)
{
  std::atomic<bool> media_state_changed (false);
  std::condition_variable cv;
  std::mutex mtx;
  std::unique_lock<std::mutex> lck (mtx);

  std::string pipelineId = createPipeline ();

  std::shared_ptr<OV3ubscriberImpl> subscriber = createOv3Subscriber (pipelineId);
  std::shared_ptr<OV3PublisherImpl> publisher = createOv3Publisher (pipelineId, screenShare);

  std::shared_ptr<MediaElementImpl> audioDecoder = createGstreamerFilter(pipelineId, "capsfilter caps=audio/x-raw");

  std::shared_ptr<CompositeImpl> compositor = createComposite (pipelineId);
  std::shared_ptr<HubPortImpl> hubportIn = createPort(compositor);
  std::shared_ptr<HubPortImpl> hubportOut = createPort(compositor);

  subscriber->connect (audioDecoder);
  audioDecoder->connect(hubportIn);
  hubportOut->connect (publisher);
  sigc::connection conn = hubportIn->signalMediaFlowInStateChanged.connect ([&] (MediaFlowInStateChanged event) {
    BOOST_TEST_MESSAGE ("OV3Subscriber flowing");
    if ((event.getMediaType()->getValue() == MediaType::VIDEO ) && ((event.getState()->getValue()) == MediaFlowState::FLOWING)) {
      media_state_changed = true;
      cv.notify_one ();
    }
  });

  if ((subscriber == nullptr) || (publisher == nullptr)) {
      BOOST_ERROR ("Could not create subscribner");
  } else if (subscriber->subscribeParticipant("7tps-vk8m", "saul", screenShare)) {
      BOOST_TEST_MESSAGE ("OV3Subscriber created");

      if (!subscriber->getIsConnected ()) {
        BOOST_ERROR("Subscribers could not connect");
      }

      publisher->publishParticipant();
      sleep(10);
      dumpPipeline (pipelineId, "ov3subscriber2.dot");
      cv.wait (lck, [&] () {
        return media_state_changed.load ();
      });
      sleep(2);
      dumpPipeline (pipelineId, "lov3ubscriber.dot");

      subscriber->disconnect (audioDecoder);
      audioDecoder->disconnect(publisher);
      //releasePassTrhough (passthrough);
      releaseGstreamerFilter(audioDecoder);
      releaseOv3Subscriber (subscriber);
      releaseOv3Publisher (publisher);
  } else  {
      BOOST_ERROR ("Could not make subscription");
      //releasePassTrhough (passthrough);
      releaseGstreamerFilter(audioDecoder);
      releaseOv3Subscriber (subscriber);
      releaseOv3Publisher (publisher);
  }
  sleep(20);
  dumpPipeline (pipelineId, "ov3subscriber_empty.dot");
  checkPipelineEmpty (pipelineId);
  releasePipeline (pipelineId);
}


static void
test_subscriber_main ()
{
  test_subscriber(false);
}

static void
test_subscriber_ss ()
{
  test_subscriber(true);
}

static void
test_republish_main ()
{
  test_republish(false);
}

static void
test_republish_ss ()
{
  test_republish(true);
}

static void 
test_mixer_sub_pub_main()
{
  test_mixer_sub_pub(false);
}


test_suite *
init_unit_test_suite ( int , char *[] )
{
  test_suite *test = BOOST_TEST_SUITE ( "ov3subscriber" );

  if (FALSE) {
  test->add (BOOST_TEST_CASE ( &test_subscriber2 ), 0, /* timeout */ 15000);
  test->add (BOOST_TEST_CASE ( &test_subscriber_ss ), 0, /* timeout */ 15000);
  test->add (BOOST_TEST_CASE ( &test_subscriber_main ), 0, /* timeout */ 15000);
  test->add (BOOST_TEST_CASE ( &test_republish_main ), 0, /* timeout */ 15000);
  test->add (BOOST_TEST_CASE ( &test_republish_ss ), 0, /* timeout */ 15000);
  }
  test->add (BOOST_TEST_CASE ( &test_mixer_sub_pub_main ), 0, /* timeout */15000);

  return test;
}