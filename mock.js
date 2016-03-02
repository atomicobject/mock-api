var http = require('http'),
  path = require('path'),
  fs = require('fs'),
  url = require('url'),
  querystring = require('querystring');

var mocks = {};

function parsePath(request) {
  return url.parse(request.url).pathname;
}

function allCalled() {
  for(var p in mocks) {
    if(mocks.hasOwnProperty(p) && !mocks[p].called) {
      return false;
    }
  }
  return true;
}

function verify(response) {
  response.writeHead(200, {'Content-Type': 'application/json'});
  response.write(JSON.stringify({allCalled: allCalled()}));
  response.end();
}

function addMock(request, response) {
  var body = '';

  request.on('data', function(chunk) {
    body += chunk.toString();
  });

  request.on('end', function() {
    var m = JSON.parse(body);
    mocks[m.url] = m;

    response.writeHead(204);
    response.end();
  });
}

function handleMock(request, response) {
  switch (request.method) {
    case 'GET':
      verify(response);
      break;
    case 'POST':
      addMock(request, response);
      break;
    case 'DELETE':
      mocks = {};
      response.writeHead(204);
      response.end();
      break;
  }
}

function matchesParams(expected, actual) {
  for(var k in expected) {
    if(expected.hasOwnProperty(k) && expected[k] !== actual[k]) {
      return false;
    }
  }

  for(var k in actual) {
    if(actual.hasOwnProperty(k) && expected[k] !== actual[k]) {
      return false;
    }
  }

  return true;
}

function matches(expected, actual) {
  var basic = expected && expected.method === actual.method;
  if(basic) {
    if(matchesParams(expected.params, querystring.parse(url.parse(actual.url).query))) {
      return true;
    } else {
      console.warn('Hit '+expected.url+' but did not match. Expected params '+JSON.stringify(expected.params)+'. '+
                   'Given params '+JSON.stringify(querystring.parse(url.parse(actual.url).query))+'.');
      return false;
    }
  } else {
    return false;
  }
}

function isMocked(request) {
  var mock = mocks[parsePath(request)];
  return mock && matches(mock, request);
}

function returnMocked(request, response) {
  var mock = mocks[parsePath(request)];
  mock.called = true;
  response.writeHeader(mock.status || 200, {'Content-Type': 'application/json'});
  response.write(mock.response.body);
  response.end();
}

function returnStatic(filename, response) {
  if(fs.statSync(filename).isDirectory()) {
    filename += '/index.html';
  }

  fs.readFile(filename, 'binary', function(err, file) {
    if(err) {
      response.writeHead(500, {'Content-Type': 'application/json'});
      response.write(JSON.stringify({error: err}));
      response.end();
    } else {
      var contentType = '';
      if (/\.css$/.test(filename)) {
        contentType = 'text/css';
      }

      response.writeHead(200, {'Content-Type': contentType});
      response.write(file, 'binary');
      response.end();
    }
  });
}

function handleRequest(staticRoot, request, response) {
  var filename = path.join(staticRoot, parsePath(request));
  fs.exists(filename, function(exists) {
    if(exists){
      returnStatic(filename, response);
    } else if (isMocked(request)) {
      returnMocked(request, response);
    } else {
      response.writeHead(404, {'Content-Type': 'application/json'});
      response.write('{}');
      response.end();
    }
  });
}

exports.startServer = function(port, staticRoot) {
  var server = http.createServer(function(request, response) {
    var p = parsePath(request);
    if (/^\/mocks$/.test(p)) {
      handleMock(request, response);
    } else if (/^\/meta$/.test(p)) {
      response.writeHead(200, {'Content-Type': 'application/json'});
      response.write(JSON.stringify(mocks));
      response.end();
    } else {
      handleRequest(staticRoot, request, response);
    }
  }).listen(port);
  console.log('Mock API server running on port ' + port);
  return server;
}
